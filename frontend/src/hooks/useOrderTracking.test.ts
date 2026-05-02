import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createElement, type ReactNode } from 'react';
import { MockWebSocket } from '@/test/mocks/websocket';
import { useOrderTracking } from './useOrderTracking';
import { api, ACCESS_TOKEN_KEY } from '@/lib/api';
import { useToast } from '@/components/ui/use-toast';

// --- Test fixtures & wrapper ---
// NB: Файл сохранён как .ts (а не .tsx) per Plan must_haves contract; используем
// React.createElement вместо JSX, чтобы не зависеть от TSX-парсера в .ts.
let originalWebSocket: typeof WebSocket;

function createWrapper() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return {
    wrapper: ({ children }: { children: ReactNode }) =>
      createElement(QueryClientProvider, { client: qc }, children),
    qc,
  };
}

function installMockWebSocket(impl: typeof WebSocket | unknown): void {
  // jsdom v26+ exposes `WebSocket` as a non-writable descriptor on globalThis,
  // so plain assignment throws. Use defineProperty to override + restore.
  Object.defineProperty(globalThis, 'WebSocket', {
    value: impl,
    writable: true,
    configurable: true,
  });
}

beforeEach(() => {
  vi.useFakeTimers();
  MockWebSocket.reset();
  originalWebSocket = globalThis.WebSocket;
  installMockWebSocket(MockWebSocket as unknown as typeof WebSocket);
  localStorage.setItem(ACCESS_TOKEN_KEY, 'tok-abc');
});

afterEach(() => {
  installMockWebSocket(originalWebSocket);
  localStorage.clear();
  vi.restoreAllMocks();
  vi.useRealTimers();
});

describe('useOrderTracking', () => {
  it('opens WebSocket with token query param', () => {
    const { wrapper } = createWrapper();
    renderHook(() => useOrderTracking('ord-1'), { wrapper });
    expect(MockWebSocket.instances).toHaveLength(1);
    // URL must match nginx /ws/ route + api-gateway /ws/orders/{id} contract
    expect(MockWebSocket.instances[0].url).toContain('/ws/orders/ord-1');
    expect(MockWebSocket.instances[0].url).toContain('token=tok-abc');
  });

  it('sets connectionState to connected on onopen', () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useOrderTracking('ord-1'), { wrapper });
    act(() => MockWebSocket.instances[0].open());
    expect(result.current.connectionState).toBe('connected');
  });

  it('dispatches location_update to lastLocation state', () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useOrderTracking('ord-1'), { wrapper });
    act(() => MockWebSocket.instances[0].open());
    act(() =>
      MockWebSocket.instances[0].emit({
        type: 'location_update',
        data: {
          courier_id: 'c',
          order_id: 'ord-1',
          lat: 55.5,
          lng: 37.5,
          timestamp: '2026-05-15T12:00:00Z',
        },
      }),
    );
    expect(result.current.lastLocation?.lat).toBe(55.5);
    expect(result.current.lastLocation?.lng).toBe(37.5);
  });

  it('appends location to history array', () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useOrderTracking('ord-1'), { wrapper });
    act(() => MockWebSocket.instances[0].open());
    act(() => {
      MockWebSocket.instances[0].emit({
        type: 'location_update',
        data: {
          courier_id: 'c',
          order_id: 'ord-1',
          lat: 55.5,
          lng: 37.5,
          timestamp: '2026-05-15T12:00:00Z',
        },
      });
      MockWebSocket.instances[0].emit({
        type: 'location_update',
        data: {
          courier_id: 'c',
          order_id: 'ord-1',
          lat: 55.6,
          lng: 37.6,
          timestamp: '2026-05-15T12:01:00Z',
        },
      });
    });
    expect(result.current.history).toHaveLength(2);
    expect(result.current.history[1].lat).toBe(55.6);
  });

  it('invalidates order query on order_status_change event', () => {
    const { wrapper, qc } = createWrapper();
    const spy = vi.spyOn(qc, 'invalidateQueries');
    renderHook(() => useOrderTracking('ord-1'), { wrapper });
    act(() => MockWebSocket.instances[0].open());
    act(() =>
      MockWebSocket.instances[0].emit({
        type: 'order_status_change',
        data: { order_id: 'ord-1', old_status: 'created', new_status: 'confirmed' },
      }),
    );
    expect(spy).toHaveBeenCalledWith({ queryKey: ['order', 'ord-1'] });
  });

  it('invalidates order+delivery queries on delivery_assigned', () => {
    const { wrapper, qc } = createWrapper();
    const spy = vi.spyOn(qc, 'invalidateQueries');
    renderHook(() => useOrderTracking('ord-1'), { wrapper });
    act(() => MockWebSocket.instances[0].open());
    act(() =>
      MockWebSocket.instances[0].emit({
        type: 'delivery_assigned',
        data: {
          delivery_id: 'dlv-1',
          order_id: 'ord-1',
          courier_id: 'c',
          eta: '2026-05-15T12:30:00Z',
        },
      }),
    );
    expect(spy).toHaveBeenCalledWith({ queryKey: ['order', 'ord-1'] });
    expect(spy).toHaveBeenCalledWith({ queryKey: ['delivery', 'dlv-1'] });
  });

  it('does not reconnect on graceful close 1000', () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useOrderTracking('ord-1'), { wrapper });
    act(() => MockWebSocket.instances[0].open());
    act(() => MockWebSocket.instances[0].closeFromServer(1000));
    act(() => {
      vi.advanceTimersByTime(2000);
    });
    expect(MockWebSocket.instances).toHaveLength(1); // no new WS
    expect(result.current.connectionState).toBe('closed');
  });

  it('reconnects on abnormal close 1006 after backoff', () => {
    const { wrapper } = createWrapper();
    renderHook(() => useOrderTracking('ord-1'), { wrapper });
    act(() => MockWebSocket.instances[0].open());
    act(() => MockWebSocket.instances[0].closeFromServer(1006));
    // backoff base = 1000ms ± 25% jitter → max 1250ms
    act(() => vi.advanceTimersByTime(1300));
    expect(MockWebSocket.instances.length).toBeGreaterThanOrEqual(2);
  });

  it('applies exponential backoff after multiple closes', () => {
    const { wrapper } = createWrapper();
    renderHook(() => useOrderTracking('ord-1'), { wrapper });
    // 1st: connect → close
    act(() => {
      MockWebSocket.instances[0].open();
      MockWebSocket.instances[0].closeFromServer(1006);
    });
    act(() => vi.advanceTimersByTime(1300)); // backoff[0]=1000
    expect(MockWebSocket.instances.length).toBe(2);
    // 2nd close → backoff[1]=2000
    act(() => {
      MockWebSocket.instances[1].open();
      MockWebSocket.instances[1].closeFromServer(1006);
    });
    act(() => vi.advanceTimersByTime(1200));
    expect(MockWebSocket.instances.length).toBe(2); // not yet
    act(() => vi.advanceTimersByTime(1500));
    expect(MockWebSocket.instances.length).toBe(3); // 2000 ± 500ms < 2700ms
  });

  it('sets offline state after 5+ failed attempts with intermediate reconnecting state', () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useOrderTracking('ord-1'), { wrapper });

    // Iter 1: connect → close 1006
    act(() => {
      MockWebSocket.instances[0].open();
      MockWebSocket.instances[0].closeFromServer(1006);
    });
    act(() => vi.advanceTimersByTime(60_000));
    expect(result.current.connectionState).toBe('reconnecting');

    // Iter 2-4: still reconnecting
    for (let i = 1; i < 4; i++) {
      const ws = MockWebSocket.instances[MockWebSocket.instances.length - 1];
      act(() => {
        ws.open();
        ws.closeFromServer(1006);
      });
      act(() => vi.advanceTimersByTime(60_000));
    }
    // After 4 attempts, should still be reconnecting (not yet offline at 5+)
    expect(result.current.connectionState).toBe('reconnecting');

    // Iter 5: triggers offline
    const ws4 = MockWebSocket.instances[MockWebSocket.instances.length - 1];
    act(() => {
      ws4.open();
      ws4.closeFromServer(1006);
    });
    act(() => vi.advanceTimersByTime(60_000));
    expect(result.current.connectionState).toBe('offline');
  });

  it('cleans up on unmount (closes WS with 1000, no reconnect)', () => {
    const { wrapper } = createWrapper();
    const { unmount } = renderHook(() => useOrderTracking('ord-1'), { wrapper });
    act(() => MockWebSocket.instances[0].open());
    const initialCount = MockWebSocket.instances.length;
    unmount();
    act(() => vi.advanceTimersByTime(60_000));
    expect(MockWebSocket.instances.length).toBe(initialCount); // no reconnect
    expect(MockWebSocket.instances[0].readyState).toBe(3); // CLOSED
  });

  it('resets backoff on online event', () => {
    const { wrapper } = createWrapper();
    renderHook(() => useOrderTracking('ord-1'), { wrapper });
    act(() => {
      MockWebSocket.instances[0].open();
      MockWebSocket.instances[0].closeFromServer(1006);
    });
    // Now a reconnect timer is pending (~1s). Fire 'online' to reset.
    act(() => window.dispatchEvent(new Event('online')));
    expect(MockWebSocket.instances.length).toBe(2); // immediate reconnect
  });

  it('sets offline state on window offline event', () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useOrderTracking('ord-1'), { wrapper });
    act(() => MockWebSocket.instances[0].open());
    act(() => window.dispatchEvent(new Event('offline')));
    expect(result.current.connectionState).toBe('offline');
  });

  it('ignores invalid envelope (logs warning, no state change)', () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {});
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useOrderTracking('ord-1'), { wrapper });
    act(() => MockWebSocket.instances[0].open());
    act(() =>
      MockWebSocket.instances[0].onmessage?.({ data: 'not-json' } as MessageEvent),
    );
    expect(warn).toHaveBeenCalled();
    expect(result.current.lastLocation).toBeNull();
  });

  it('refreshes token and reconnects on close 1008 (auth-class)', async () => {
    // Per D-11: close 1008/4401 → refresh access_token → new WS with updated token in URL.
    const { wrapper } = createWrapper();

    // Mock api.get('/auth/refresh') to succeed and update localStorage with new token.
    const refreshSpy = vi.spyOn(api, 'get').mockImplementation(async (path: string) => {
      if (path === '/auth/refresh' || path.includes('/auth/refresh')) {
        localStorage.setItem(ACCESS_TOKEN_KEY, 'tok-NEW');
        return { data: { access_token: 'tok-NEW' } } as never;
      }
      return { data: {} } as never;
    });

    renderHook(() => useOrderTracking('ord-1'), { wrapper });
    act(() => MockWebSocket.instances[0].open());

    // Server closes with 1008 (policy violation — auth class)
    act(() => MockWebSocket.instances[0].closeFromServer(1008));

    // Allow async refresh + scheduled reconnect to fire
    await act(async () => {
      await Promise.resolve(); // flush microtasks for refresh.then
      await Promise.resolve();
      vi.advanceTimersByTime(1300); // backoff window (jitter ±25%)
    });

    expect(refreshSpy).toHaveBeenCalled();
    expect(MockWebSocket.instances.length).toBeGreaterThanOrEqual(2);
    // New WS URL should contain the refreshed token
    const latestWs = MockWebSocket.instances[MockWebSocket.instances.length - 1];
    expect(latestWs.url).toContain('token=tok-NEW');
  });

  it('does not reconnect when refresh fails on close 1008', async () => {
    const { wrapper } = createWrapper();

    // Mock refresh to reject — interceptor in lib/api would normally dispatch logout
    const refreshSpy = vi.spyOn(api, 'get').mockImplementation(async (path: string) => {
      if (path === '/auth/refresh' || path.includes('/auth/refresh')) {
        throw new Error('refresh_failed');
      }
      return { data: {} } as never;
    });

    const { result } = renderHook(() => useOrderTracking('ord-1'), { wrapper });
    act(() => MockWebSocket.instances[0].open());
    const initialCount = MockWebSocket.instances.length;

    act(() => MockWebSocket.instances[0].closeFromServer(1008));

    await act(async () => {
      await Promise.resolve();
      await Promise.resolve(); // multiple flushes for promise rejection
      vi.advanceTimersByTime(2000);
    });

    expect(refreshSpy).toHaveBeenCalled();
    // No new WS instance created — refresh failed, hook gave up
    expect(MockWebSocket.instances.length).toBe(initialCount);
    // State should be 'closed' (logout path) or 'offline'
    expect(['closed', 'offline']).toContain(result.current.connectionState);
  });
});

describe('useOrderTracking — PLSH-03 status toast', () => {
  // Helper: clear module-scope toast memoryState between tests.
  // dismiss() marks toasts open=false but REMOVE_TOAST fires after TOAST_REMOVE_DELAY (1_000_000ms).
  // With fake timers we advance past TOAST_REMOVE_DELAY to flush removal from memoryState.
  async function clearToasts() {
    const { result } = renderHook(() => useToast());
    await act(async () => {
      result.current.dismiss();
    });
    // Advance fake timers past TOAST_REMOVE_DELAY=1_000_000ms so REMOVE_TOAST fires.
    act(() => vi.advanceTimersByTime(1_100_000));
  }

  beforeEach(async () => {
    vi.useFakeTimers();
    MockWebSocket.reset();
    originalWebSocket = globalThis.WebSocket;
    installMockWebSocket(MockWebSocket as unknown as typeof WebSocket);
    localStorage.setItem(ACCESS_TOKEN_KEY, 'tok-abc');
    await clearToasts();
  });

  afterEach(async () => {
    await clearToasts();
    installMockWebSocket(originalWebSocket);
    localStorage.clear();
    vi.restoreAllMocks();
    vi.useRealTimers();
  });

  it('fires toast "Курьер назначен" on order_status_change new_status=assigned', async () => {
    const { wrapper } = createWrapper();
    renderHook(() => useOrderTracking('ord-1'), { wrapper });
    act(() => MockWebSocket.instances[0].open());

    const { result: toastResult } = renderHook(() => useToast());
    await act(async () => {
      MockWebSocket.instances[0].emit({
        type: 'order_status_change',
        data: { order_id: 'ord-1', old_status: 'confirmed', new_status: 'assigned' },
      });
    });

    expect(toastResult.current.toasts[0]?.title).toBe('Курьер назначен');
  });

  it('fires toast "Курьер забрал ваш заказ" on order_status_change new_status=picked_up', async () => {
    const { wrapper } = createWrapper();
    renderHook(() => useOrderTracking('ord-1'), { wrapper });
    act(() => MockWebSocket.instances[0].open());

    const { result: toastResult } = renderHook(() => useToast());
    await act(async () => {
      MockWebSocket.instances[0].emit({
        type: 'order_status_change',
        data: { order_id: 'ord-1', old_status: 'assigned', new_status: 'picked_up' },
      });
    });

    expect(toastResult.current.toasts[0]?.title).toBe('Курьер забрал ваш заказ');
  });

  it('fires toast "Курьер в пути" on order_status_change new_status=in_transit', async () => {
    const { wrapper } = createWrapper();
    renderHook(() => useOrderTracking('ord-1'), { wrapper });
    act(() => MockWebSocket.instances[0].open());

    const { result: toastResult } = renderHook(() => useToast());
    await act(async () => {
      MockWebSocket.instances[0].emit({
        type: 'order_status_change',
        data: { order_id: 'ord-1', old_status: 'picked_up', new_status: 'in_transit' },
      });
    });

    expect(toastResult.current.toasts[0]?.title).toBe('Курьер в пути');
  });

  it('fires toast "Заказ доставлен" on order_status_change new_status=delivered', async () => {
    const { wrapper } = createWrapper();
    renderHook(() => useOrderTracking('ord-1'), { wrapper });
    act(() => MockWebSocket.instances[0].open());

    const { result: toastResult } = renderHook(() => useToast());
    await act(async () => {
      MockWebSocket.instances[0].emit({
        type: 'order_status_change',
        data: { order_id: 'ord-1', old_status: 'in_transit', new_status: 'delivered' },
      });
    });

    expect(toastResult.current.toasts[0]?.title).toBe('Заказ доставлен');
  });

  it('fires toast "Заказ отменён" on order_status_change new_status=cancelled', async () => {
    const { wrapper } = createWrapper();
    renderHook(() => useOrderTracking('ord-1'), { wrapper });
    act(() => MockWebSocket.instances[0].open());

    const { result: toastResult } = renderHook(() => useToast());
    await act(async () => {
      MockWebSocket.instances[0].emit({
        type: 'order_status_change',
        data: { order_id: 'ord-1', old_status: 'in_transit', new_status: 'cancelled' },
      });
    });

    expect(toastResult.current.toasts[0]?.title).toBe('Заказ отменён');
  });

  it('does NOT fire toast for new_status=created (noise suppressed)', async () => {
    const { wrapper } = createWrapper();
    renderHook(() => useOrderTracking('ord-1'), { wrapper });
    act(() => MockWebSocket.instances[0].open());

    const { result: toastResult } = renderHook(() => useToast());
    await act(async () => {
      MockWebSocket.instances[0].emit({
        type: 'order_status_change',
        data: { order_id: 'ord-1', old_status: '', new_status: 'created' },
      });
    });

    // No status-copy toast should have been added — check none of the curated titles present
    const titles = toastResult.current.toasts.map((t) => t.title);
    expect(titles).not.toContain('Курьер назначен');
    expect(titles).not.toContain('Курьер в пути');
    expect(titles).not.toContain('Заказ доставлен');
    expect(titles).not.toContain('Заказ отменён');
  });

  it('does NOT fire toast for new_status=confirmed (noise suppressed)', async () => {
    const { wrapper } = createWrapper();
    renderHook(() => useOrderTracking('ord-1'), { wrapper });
    act(() => MockWebSocket.instances[0].open());

    const { result: toastResult } = renderHook(() => useToast());
    await act(async () => {
      MockWebSocket.instances[0].emit({
        type: 'order_status_change',
        data: { order_id: 'ord-1', old_status: 'created', new_status: 'confirmed' },
      });
    });

    const titles = toastResult.current.toasts.map((t) => t.title);
    expect(titles).not.toContain('Курьер назначен');
    expect(titles).not.toContain('Курьер в пути');
    expect(titles).not.toContain('Заказ доставлен');
    expect(titles).not.toContain('Заказ отменён');
  });

  it('fires toast "Заказ доставлен" for ORDER_STATUS_DELIVERED (defensive normalization)', async () => {
    const { wrapper } = createWrapper();
    renderHook(() => useOrderTracking('ord-1'), { wrapper });
    act(() => MockWebSocket.instances[0].open());

    const { result: toastResult } = renderHook(() => useToast());
    await act(async () => {
      MockWebSocket.instances[0].emit({
        type: 'order_status_change',
        data: { order_id: 'ord-1', old_status: 'ORDER_STATUS_IN_TRANSIT', new_status: 'ORDER_STATUS_DELIVERED' },
      });
    });

    expect(toastResult.current.toasts[0]?.title).toBe('Заказ доставлен');
  });
});
