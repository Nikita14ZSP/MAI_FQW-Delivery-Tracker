import { describe, it, expect } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { QueryClient } from '@tanstack/react-query';
import { server } from '@/test/mocks/server';
import { renderWithProviders } from '@/test/test-utils';
import { AvailableOrdersTab } from './AvailableOrdersTab';

function makeQC() {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
}

describe('AvailableOrdersTab', () => {
  it('offline gate: shows offline message and no cards when isOnline=false', () => {
    renderWithProviders(<AvailableOrdersTab isOnline={false} />, {
      queryClient: makeQC(),
    });

    expect(
      screen.getByText(/Переключитесь в режим/i),
    ).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Принять' })).toBeNull();
  });

  it('renders available order card when isOnline=true', async () => {
    renderWithProviders(<AvailableOrdersTab isOnline={true} />, {
      queryClient: makeQC(),
    });

    await waitFor(() => {
      expect(screen.getByText('Москва, Тверская, 1')).toBeInTheDocument();
    });
  });

  it('optimistic accept: card disappears immediately on click and shows success toast path', async () => {
    const qc = makeQC();
    renderWithProviders(<AvailableOrdersTab isOnline={true} />, {
      queryClient: qc,
    });

    // Wait for card to appear
    await waitFor(() =>
      expect(screen.getByText('Москва, Тверская, 1')).toBeInTheDocument(),
    );

    // Click Принять — optimistic removal
    await userEvent.click(screen.getByRole('button', { name: 'Принять' }));

    // Card should disappear from DOM (optimistic remove)
    await waitFor(() =>
      expect(
        screen.queryByText('Москва, Тверская, 1'),
      ).not.toBeInTheDocument(),
    );
  });

  it('rollback on 409: card reappears after max_active_reached rejection', async () => {
    server.use(
      http.post(
        '*/v1/couriers/me/deliveries/:id/accept',
        () =>
          HttpResponse.json(
            { code: 10, message: 'max_active_reached' },
            { status: 409 },
          ),
      ),
    );

    const qc = makeQC();
    renderWithProviders(<AvailableOrdersTab isOnline={true} />, {
      queryClient: qc,
    });

    // Wait for card to appear
    await waitFor(() =>
      expect(screen.getByText('Москва, Тверская, 1')).toBeInTheDocument(),
    );

    // Click Принять
    await userEvent.click(screen.getByRole('button', { name: 'Принять' }));

    // After 409 rejection, card should reappear (rollback)
    await waitFor(() =>
      expect(screen.getByText('Москва, Тверская, 1')).toBeInTheDocument(),
    );
  });

  it('empty state: shows "Нет доступных заказов" when orders list is empty', async () => {
    server.use(
      http.get('*/v1/couriers/me/available-orders', () =>
        HttpResponse.json({ orders: [], nextCursor: '' }),
      ),
    );

    renderWithProviders(<AvailableOrdersTab isOnline={true} />, {
      queryClient: makeQC(),
    });

    await waitFor(() =>
      expect(screen.getByText(/Нет доступных заказов/i)).toBeInTheDocument(),
    );
  });
});
