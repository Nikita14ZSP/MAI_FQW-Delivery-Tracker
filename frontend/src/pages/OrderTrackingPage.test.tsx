import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, act } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { http, HttpResponse } from 'msw';
import { server } from '@/test/mocks/server';
import { MockWebSocket } from '@/test/mocks/websocket';

const mockNavigate = vi.fn();
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom');
  return { ...actual, useNavigate: () => mockNavigate };
});

// Mock TrackingMap to expose props for assertion (Phase 8 pattern)
const mockTrackingMapProps = vi.fn();
vi.mock('@/components/tracking/TrackingMap', () => ({
  TrackingMap: (props: {
    destination?: { lat: number; lng: number };
    lastLocation?: { lat: number; lng: number } | null;
    history?: Array<unknown>;
  }) => {
    mockTrackingMapProps(props);
    return (
      <div
        data-testid="tracking-map-stub"
        data-dest-lat={props.destination?.lat}
        data-dest-lng={props.destination?.lng}
        data-last-lat={props.lastLocation?.lat ?? ''}
        data-history-count={props.history?.length ?? 0}
      />
    );
  },
}));

// Helper: build handler for /v1/orders/:id with overrides
function makeOrderHandler(overrides: Record<string, unknown> = {}) {
  return http.get('*/v1/orders/:id', () =>
    HttpResponse.json({
      order: {
        id: 'ord-1',
        userId: 'u-1',
        status: 'ORDER_STATUS_CREATED',
        delivery_id: 'dlv-mock-1',
        deliveryAddress: 'Москва, ул. Тверская, 1',
        deliveryCoordinates: { latitude: 55.7558, longitude: 37.6173 },
        items: [],
        contactPhone: '+71234567890',
        paymentMethod: 'cash',
        createdAt: '2026-05-15T11:00:00Z',
        updatedAt: '2026-05-15T11:00:00Z',
        ...overrides,
      },
    }),
  );
}

async function importPage() {
  const mod = await import('./OrderTrackingPage');
  return mod.OrderTrackingPage;
}

function renderPage(OrderTrackingPage: React.ComponentType, orderId = 'ord-1') {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={[`/orders/${orderId}/track`]}>
        <Routes>
          <Route path="/orders/:id/track" element={<OrderTrackingPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  mockNavigate.mockClear();
  mockTrackingMapProps.mockClear();
});
afterEach(() => {
  vi.useRealTimers();
  server.resetHandlers();
  vi.doUnmock('@/hooks/useOrderTracking');
});

// --- Group A: useOrderTracking MOCKED — fast layout + edge state tests ---
describe('OrderTrackingPage (with useOrderTracking mocked)', () => {
  beforeEach(() => {
    vi.doMock('@/hooks/useOrderTracking', () => ({
      useOrderTracking: () => ({
        connectionState: 'connected',
        lastLocation: null,
        history: [],
      }),
    }));
    vi.resetModules();
  });

  it('shows WaitingForCourier when status is "created" (no courier assigned)', async () => {
    server.use(makeOrderHandler({ status: 'ORDER_STATUS_CREATED' }));
    // delivery query may return 404 since no assigned courier; handler defaults to one — override
    server.use(
      http.get('*/v1/deliveries/by-order/:orderId', () =>
        HttpResponse.json({ error: 'not found' }, { status: 404 }),
      ),
    );
    const OrderTrackingPage = await importPage();
    renderPage(OrderTrackingPage);
    await waitFor(() => expect(screen.getByText('Поиск курьера...')).toBeInTheDocument());
  });

  it('passes destination from order.deliveryCoordinates to TrackingMap as {lat, lng}', async () => {
    server.use(
      makeOrderHandler({
        status: 'ORDER_STATUS_ASSIGNED',
        delivery_id: 'dlv-1',
        deliveryCoordinates: { latitude: 55.7558, longitude: 37.6173 },
      }),
    );
    const OrderTrackingPage = await importPage();
    renderPage(OrderTrackingPage);
    await waitFor(() => expect(screen.getByTestId('tracking-map-stub')).toBeInTheDocument());
    const mapStub = screen.getByTestId('tracking-map-stub');
    expect(mapStub).toHaveAttribute('data-dest-lat', '55.7558');
    expect(mapStub).toHaveAttribute('data-dest-lng', '37.6173');
  });

  it('redirects to /orders/:id when status is cancelled', async () => {
    server.use(makeOrderHandler({ status: 'ORDER_STATUS_CANCELLED' }));
    const OrderTrackingPage = await importPage();
    renderPage(OrderTrackingPage);
    await waitFor(() => expect(mockNavigate).toHaveBeenCalledWith('/orders/ord-1', { replace: true }));
  });

  it('redirects to /orders/:id when status is failed', async () => {
    server.use(makeOrderHandler({ status: 'ORDER_STATUS_FAILED' }));
    const OrderTrackingPage = await importPage();
    renderPage(OrderTrackingPage);
    await waitFor(() => expect(mockNavigate).toHaveBeenCalledWith('/orders/ord-1', { replace: true }));
  });

  it('redirects to /orders/:id when status is returned', async () => {
    server.use(makeOrderHandler({ status: 'ORDER_STATUS_RETURNED' }));
    const OrderTrackingPage = await importPage();
    renderPage(OrderTrackingPage);
    await waitFor(() => expect(mockNavigate).toHaveBeenCalledWith('/orders/ord-1', { replace: true }));
  });

  it('shows delivered overlay then redirects after 3s when status is delivered', async () => {
    server.use(makeOrderHandler({ status: 'ORDER_STATUS_DELIVERED' }));
    const OrderTrackingPage = await importPage();
    vi.useFakeTimers({ shouldAdvanceTime: true });
    renderPage(OrderTrackingPage);
    await waitFor(() => expect(screen.getByText('Заказ доставлен!')).toBeInTheDocument());
    await act(async () => {
      await vi.advanceTimersByTimeAsync(3100);
    });
    expect(mockNavigate).toHaveBeenCalledWith('/orders/ord-1', { replace: true });
  });

  it('renders TrackingMap + StatusBadge when status is assigned with courier', async () => {
    server.use(makeOrderHandler({ status: 'ORDER_STATUS_ASSIGNED', delivery_id: 'dlv-1' }));
    const OrderTrackingPage = await importPage();
    renderPage(OrderTrackingPage);
    await waitFor(() => expect(screen.getByTestId('tracking-map-stub')).toBeInTheDocument());
    // StatusBadge for 'assigned' — Russian label per ORDER_STATUS_LABELS
    expect(screen.getByText('Назначен курьеру')).toBeInTheDocument();
  });
});

// --- Group B: useOrderTracking REAL (uses MockWebSocket) — integration wiring test ---
describe('OrderTrackingPage (integration with real useOrderTracking + MockWebSocket)', () => {
  let originalWebSocketDesc: PropertyDescriptor | undefined;

  beforeEach(() => {
    MockWebSocket.reset();
    originalWebSocketDesc = Object.getOwnPropertyDescriptor(globalThis, 'WebSocket');
    Object.defineProperty(globalThis, 'WebSocket', {
      value: MockWebSocket,
      writable: true,
      configurable: true,
    });
    localStorage.setItem('access_token', 'tok-test');
    vi.resetModules();
  });

  afterEach(() => {
    if (originalWebSocketDesc) {
      Object.defineProperty(globalThis, 'WebSocket', originalWebSocketDesc);
    } else {
      delete (globalThis as { WebSocket?: unknown }).WebSocket;
    }
    localStorage.clear();
  });

  it('wires WS location_update → TrackingMap lastLocation prop (no useOrderTracking mock)', async () => {
    server.use(
      makeOrderHandler({ status: 'ORDER_STATUS_ASSIGNED', delivery_id: 'dlv-1' }),
    );

    const OrderTrackingPage = await importPage();
    renderPage(OrderTrackingPage);

    // Wait for page to render TrackingMap (after orderQuery resolves)
    await waitFor(() => expect(screen.getByTestId('tracking-map-stub')).toBeInTheDocument());

    // Wait for WS to be constructed
    await waitFor(() => expect(MockWebSocket.instances.length).toBeGreaterThanOrEqual(1));
    await act(async () => {
      MockWebSocket.instances[0].open();
    });

    // Emit a location_update WS event — hook should propagate to TrackingMap props
    await act(async () => {
      MockWebSocket.instances[0].emit({
        type: 'location_update',
        data: {
          courier_id: 'crr-1',
          order_id: 'ord-1',
          lat: 55.76,
          lng: 37.62,
          timestamp: '2026-05-15T12:05:00Z',
        },
      });
    });

    // TrackingMap stub should receive updated lastLocation
    await waitFor(() => {
      const mapStub = screen.getByTestId('tracking-map-stub');
      expect(mapStub).toHaveAttribute('data-last-lat', '55.76');
    });
  });
});

// --- CTRK-04: courier ФИО + ★ from public profile (replaces «Курьер #xxxx») ---
describe('CTRK-04 courier profile', () => {
  beforeEach(() => {
    vi.doMock('@/hooks/useOrderTracking', () => ({
      useOrderTracking: () => ({
        connectionState: 'connected',
        lastLocation: null,
        history: [],
      }),
    }));
    vi.resetModules();
  });

  it('shows ФИО + ★ from profile, not «Курьер #xxxx»', async () => {
    server.use(
      makeOrderHandler({ status: 'ORDER_STATUS_ASSIGNED', delivery_id: 'dlv-1' }),
    );
    const OrderTrackingPage = await importPage();
    renderPage(OrderTrackingPage);

    expect(await screen.findByText('Иван Иванов')).toBeInTheDocument();
    expect(screen.getByText('★ 4.5')).toBeInTheDocument();
    expect(screen.queryByText(/Курьер #/)).not.toBeInTheDocument();
  });

  it('falls back to «Курьер» (no #id, no crash) when profile 404s (D-09)', async () => {
    server.use(
      makeOrderHandler({ status: 'ORDER_STATUS_ASSIGNED', delivery_id: 'dlv-1' }),
      http.get('*/v1/couriers/:courierId/profile', () =>
        HttpResponse.json({ error: 'not found' }, { status: 404 }),
      ),
    );
    const OrderTrackingPage = await importPage();
    renderPage(OrderTrackingPage);

    // tracking UI still renders + courier label degrades to plain «Курьер»
    await waitFor(() =>
      expect(screen.getByTestId('tracking-map-stub')).toBeInTheDocument(),
    );
    // CourierInfo renders the section label «Курьер» AND the fallback name
    // «Курьер» (no #id) — both present, page did not crash (D-09).
    await waitFor(() =>
      expect(screen.getAllByText('Курьер').length).toBeGreaterThanOrEqual(2),
    );
    expect(screen.queryByText(/Курьер #/)).not.toBeInTheDocument();
    expect(screen.queryByText(/★/)).not.toBeInTheDocument();
  });
});
