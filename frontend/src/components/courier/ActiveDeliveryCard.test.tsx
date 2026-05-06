/**
 * ActiveDeliveryCard.test.tsx — TDD RED → GREEN for the active delivery card component.
 * Uses renderWithProviders with courier user context and MSW for API mocking.
 */
import { describe, it, expect, vi } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { server } from '@/test/mocks/server';
import { QueryClient } from '@tanstack/react-query';
import { renderWithProviders } from '@/test/test-utils';
import { ActiveDeliveryCard } from './ActiveDeliveryCard';
import type { Delivery } from '@/types/tracking';

const BASE = '/v1';

const mockUser = {
  id: 'crr-1',
  email: 'courier@test.com',
  first_name: 'Иван',
  last_name: 'К',
  role: 'courier' as const,
};

// Sample order returned by GET /v1/orders/:id
const mockOrderRaw = {
  id: 'ord-act-1',
  userId: 'u',
  status: 'ORDER_STATUS_ASSIGNED',
  deliveryAddress: 'Москва, Тверская, 1',
  deliveryCoordinates: { latitude: 55.7600, longitude: 37.6200 },
  items: [],
  contactPhone: '+79991234567',
  paymentMethod: 'card_on_delivery',
  createdAt: '2026-05-16T07:00:00Z',
  updatedAt: '2026-05-16T07:00:00Z',
  delivery_id: 'dlv-act-1',
};

// Add getOrder handler to MSW
function addOrderHandler() {
  server.use(
    http.get(`${BASE}/orders/:id`, () =>
      HttpResponse.json({ order: mockOrderRaw }),
    ),
  );
}

function freshQueryClient() {
  return new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
}

function makeDelivery(status: string): Delivery {
  return {
    id: 'dlv-act-1',
    order_id: 'ord-act-1',
    courier_id: 'crr-1',
    zone_id: 'zone-1',
    status,
    estimated_delivery: '2026-05-16T08:30:00Z',
  };
}

describe('ActiveDeliveryCard', () => {
  it('1. status assigned → next-stage button «Забрал заказ», clicking calls onAdvance with ORDER_STATUS_PICKED_UP', async () => {
    addOrderHandler();
    const onAdvance = vi.fn();

    renderWithProviders(
      <ActiveDeliveryCard
        delivery={makeDelivery('assigned')}
        isOnline={true}
        onAdvance={onAdvance}
        isAdvancing={false}
      />,
      { user: mockUser, queryClient: freshQueryClient() },
    );

    const btn = screen.getByRole('button', { name: 'Забрал заказ' });
    expect(btn).toBeInTheDocument();

    await userEvent.click(btn);
    expect(onAdvance).toHaveBeenCalledWith('ord-act-1', 'ORDER_STATUS_PICKED_UP');
  });

  it('2. status picked_up → «Начать доставку» / ORDER_STATUS_IN_TRANSIT; in_transit → «Доставлено» / ORDER_STATUS_DELIVERED', async () => {
    addOrderHandler();
    const onAdvance1 = vi.fn();
    const onAdvance2 = vi.fn();

    const { unmount } = renderWithProviders(
      <ActiveDeliveryCard
        delivery={makeDelivery('picked_up')}
        isOnline={true}
        onAdvance={onAdvance1}
        isAdvancing={false}
      />,
      { user: mockUser, queryClient: freshQueryClient() },
    );

    const btn1 = screen.getByRole('button', { name: 'Начать доставку' });
    await userEvent.click(btn1);
    expect(onAdvance1).toHaveBeenCalledWith('ord-act-1', 'ORDER_STATUS_IN_TRANSIT');
    unmount();

    renderWithProviders(
      <ActiveDeliveryCard
        delivery={makeDelivery('in_transit')}
        isOnline={true}
        onAdvance={onAdvance2}
        isAdvancing={false}
      />,
      { user: mockUser, queryClient: freshQueryClient() },
    );

    const btn2 = screen.getByRole('button', { name: 'Доставлено' });
    await userEvent.click(btn2);
    expect(onAdvance2).toHaveBeenCalledWith('ord-act-1', 'ORDER_STATUS_DELIVERED');
  });

  it('3. status delivered → no next-stage button rendered', () => {
    addOrderHandler();

    renderWithProviders(
      <ActiveDeliveryCard
        delivery={makeDelivery('delivered')}
        isOnline={true}
        onAdvance={vi.fn()}
        isAdvancing={false}
      />,
      { user: mockUser, queryClient: freshQueryClient() },
    );

    expect(screen.queryByRole('button', { name: 'Забрал заказ' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Начать доставку' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Доставлено' })).not.toBeInTheDocument();
  });

  it('4. isOnline=true + picked_up + order coords loaded → «Начать движение» present; isOnline=false → not rendered', async () => {
    addOrderHandler();

    const { unmount } = renderWithProviders(
      <ActiveDeliveryCard
        delivery={makeDelivery('picked_up')}
        isOnline={true}
        onAdvance={vi.fn()}
        isAdvancing={false}
      />,
      { user: mockUser, queryClient: freshQueryClient() },
    );

    // Wait for address to load (order query resolves)
    await waitFor(() => {
      expect(screen.getByText('Москва, Тверская, 1')).toBeInTheDocument();
    });

    expect(screen.getByRole('button', { name: /Начать движение/i })).toBeInTheDocument();
    unmount();

    // Now test with isOnline=false
    renderWithProviders(
      <ActiveDeliveryCard
        delivery={makeDelivery('picked_up')}
        isOnline={false}
        onAdvance={vi.fn()}
        isAdvancing={false}
      />,
      { user: mockUser, queryClient: freshQueryClient() },
    );

    // When offline, GPS block not shown
    expect(screen.queryByRole('button', { name: /Начать движение/i })).not.toBeInTheDocument();
  });

  it('5. isAdvancing=true → next-stage button disabled', () => {
    addOrderHandler();

    renderWithProviders(
      <ActiveDeliveryCard
        delivery={makeDelivery('assigned')}
        isOnline={true}
        onAdvance={vi.fn()}
        isAdvancing={true}
      />,
      { user: mockUser, queryClient: freshQueryClient() },
    );

    // When advancing, the button should be disabled (shows spinner)
    const btn = screen.getByRole('button');
    expect(btn).toBeDisabled();
  });
});
