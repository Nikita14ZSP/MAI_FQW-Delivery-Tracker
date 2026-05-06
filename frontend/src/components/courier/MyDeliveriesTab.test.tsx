/**
 * MyDeliveriesTab.test.tsx — TDD RED → GREEN for the "Мои доставки" tab.
 * Uses renderWithProviders with courier user context and MSW for API mocking.
 */
import { describe, it, expect } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { server } from '@/test/mocks/server';
import { QueryClient } from '@tanstack/react-query';
import { renderWithProviders } from '@/test/test-utils';
import { MyDeliveriesTab } from './MyDeliveriesTab';

const BASE = '/v1';

const mockUser = {
  id: 'crr-1',
  email: 'courier@test.com',
  first_name: 'И',
  last_name: 'К',
  role: 'courier' as const,
};

// Default order handler: GET /v1/orders/:id → returns address + coords
function addOrderHandler() {
  server.use(
    http.get(`${BASE}/orders/:id`, () =>
      HttpResponse.json({
        order: {
          id: 'ord-act-1',
          userId: 'u',
          status: 'ORDER_STATUS_ASSIGNED',
          deliveryAddress: 'Москва, Тверская, 1',
          deliveryCoordinates: { latitude: 55.76, longitude: 37.62 },
          items: [],
          contactPhone: '+79991234567',
          paymentMethod: 'card_on_delivery',
          createdAt: '2026-05-16T07:00:00Z',
          updatedAt: '2026-05-16T07:00:00Z',
          delivery_id: 'dlv-act-1',
        },
      }),
    ),
  );
}

function freshQueryClient() {
  return new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
}

describe('MyDeliveriesTab', () => {
  it('1. isOnline=true → renders active delivery with address and «Забрал заказ» button (assigned)', async () => {
    // courierHandlers already provides GET /couriers/:courierId/deliveries → DELIVERY_STATUS_ASSIGNED 'dlv-act-1'
    addOrderHandler();

    renderWithProviders(
      <MyDeliveriesTab isOnline={true} />,
      { user: mockUser, queryClient: freshQueryClient() },
    );

    // Wait for address to appear after MSW resolves
    await waitFor(() => {
      expect(screen.getByText('Москва, Тверская, 1')).toBeInTheDocument();
    });

    expect(screen.getByRole('button', { name: 'Забрал заказ' })).toBeInTheDocument();
  });

  it('2. empty deliveries → shows «У вас нет активных доставок»', async () => {
    server.use(
      http.get(`${BASE}/couriers/:courierId/deliveries`, () =>
        HttpResponse.json({ deliveries: [] }),
      ),
    );

    renderWithProviders(
      <MyDeliveriesTab isOnline={true} />,
      { user: mockUser, queryClient: freshQueryClient() },
    );

    await waitFor(() => {
      expect(screen.getByText('У вас нет активных доставок')).toBeInTheDocument();
    });
  });

  it('3. delivered deliveries are filtered out → shows empty-state', async () => {
    server.use(
      http.get(`${BASE}/couriers/:courierId/deliveries`, () =>
        HttpResponse.json({
          deliveries: [
            {
              id: 'd1',
              orderId: 'o1',
              courierId: 'crr-1',
              status: 'DELIVERY_STATUS_DELIVERED',
              createdAt: '2026-05-16T07:00:00Z',
            },
          ],
        }),
      ),
    );

    renderWithProviders(
      <MyDeliveriesTab isOnline={true} />,
      { user: mockUser, queryClient: freshQueryClient() },
    );

    await waitFor(() => {
      expect(screen.getByText('У вас нет активных доставок')).toBeInTheDocument();
    });
  });

  it('4. clicking advance button calls updateOrderStatus (PATCH /orders/:id/status) with ORDER_STATUS_PICKED_UP', async () => {
    addOrderHandler();
    let patchBody: unknown = null;

    server.use(
      http.patch(`${BASE}/orders/:id/status`, async ({ request, params }) => {
        patchBody = await request.json();
        return HttpResponse.json({
          order: {
            id: params.id,
            userId: 'user-1',
            status: (patchBody as { status: string }).status,
            deliveryAddress: 'Москва, Тверская, 1',
            deliveryCoordinates: { latitude: 55.7558, longitude: 37.6173 },
            items: [],
            contactPhone: '+79991234567',
            paymentMethod: 'card_on_delivery',
            createdAt: '2026-05-16T07:00:00Z',
            updatedAt: '2026-05-16T08:00:00Z',
            delivery_id: 'dlv-act-1',
          },
        });
      }),
    );

    renderWithProviders(
      <MyDeliveriesTab isOnline={true} />,
      { user: mockUser, queryClient: freshQueryClient() },
    );

    // Wait for card to appear
    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Забрал заказ' })).toBeInTheDocument();
    });

    await userEvent.click(screen.getByRole('button', { name: 'Забрал заказ' }));

    await waitFor(() => {
      expect(patchBody).toMatchObject({ status: 'ORDER_STATUS_PICKED_UP' });
    });
  });

  it('5. isOnline=false → tab still renders delivery (NOT offline-gated, D-06)', async () => {
    addOrderHandler();

    renderWithProviders(
      <MyDeliveriesTab isOnline={false} />,
      { user: mockUser, queryClient: freshQueryClient() },
    );

    // The tab should NOT show the "offline" gate message
    // and SHOULD show deliveries
    await waitFor(() => {
      expect(screen.getByText('Москва, Тверская, 1')).toBeInTheDocument();
    });

    // Verify the offline gate from AvailableOrdersTab is NOT here
    expect(screen.queryByText('Переключитесь в режим')).not.toBeInTheDocument();
  });
});
