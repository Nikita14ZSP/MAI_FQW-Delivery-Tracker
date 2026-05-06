import { describe, it, expect } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient } from '@tanstack/react-query';
import { http, HttpResponse } from 'msw';
import { server } from '@/test/mocks/server';
import { renderWithProviders } from '@/test/test-utils';
import { CourierDashboardPage } from './CourierDashboardPage';
import { AppRoutes } from '@/router';

const courierUser = {
  id: 'crr-1',
  first_name: 'Иван',
  last_name: 'Курьеров',
  role: 'courier' as const,
  email: '',
};

function makeQc() {
  return new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
}

// Extra stub so MyDeliveriesTab's ActiveDeliveryCard can resolve the linked order
const orderDetailHandler = http.get('*/v1/orders/:id', () =>
  HttpResponse.json({
    order: {
      id: 'ord-act-1',
      userId: 'u',
      status: 'ASSIGNED',
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
);

describe('CourierDashboardPage', () => {
  it('renders the header (courier name) and both tab triggers', async () => {
    server.use(orderDetailHandler);
    renderWithProviders(<CourierDashboardPage />, {
      user: courierUser,
      queryClient: makeQc(),
    });

    await waitFor(() => {
      expect(screen.getByText('Иван Курьеров')).toBeInTheDocument();
    });
    expect(screen.getByRole('tab', { name: 'Доступные заказы' })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: 'Мои доставки' })).toBeInTheDocument();
  });

  it('default tab is «Доступные заказы» and shows offline gate while offline', async () => {
    server.use(orderDetailHandler);
    renderWithProviders(<CourierDashboardPage />, {
      user: courierUser,
      queryClient: makeQc(),
    });

    await waitFor(() => {
      expect(
        screen.getByText(/Переключитесь в режим/i),
      ).toBeInTheDocument();
    });
  });

  it('toggling online lifts state so AvailableOrdersTab shows orders', async () => {
    server.use(orderDetailHandler);
    const user = userEvent.setup();
    renderWithProviders(<CourierDashboardPage />, {
      user: courierUser,
      queryClient: makeQc(),
    });

    // Wait for the "Не работаю" button to appear (header rendered)
    const toggleBtn = await screen.findByRole('button', { name: /Не работаю/i });
    await user.click(toggleBtn);

    // After toggle MSW echoes status:'available' → isOnline becomes true
    await waitFor(() => {
      expect(screen.getByText('Москва, Тверская, 1')).toBeInTheDocument();
    });
    // Offline gate must be gone
    expect(screen.queryByText(/Переключитесь в режим/i)).not.toBeInTheDocument();
  });

  it('«Мои доставки» tab renders delivery card and is NOT offline-gated', async () => {
    server.use(orderDetailHandler);
    const user = userEvent.setup();
    renderWithProviders(<CourierDashboardPage />, {
      user: courierUser,
      queryClient: makeQc(),
    });

    // Switch to «Мои доставки» tab — must be visible even offline
    const myTab = await screen.findByRole('tab', { name: 'Мои доставки' });
    await user.click(myTab);

    // ActiveDeliveryCard resolves address from the order stub
    await waitFor(() => {
      expect(screen.getByText('Москва, Тверская, 1')).toBeInTheDocument();
    });
    // No offline gate message for this tab
    expect(screen.queryByText(/Переключитесь в режим/i)).not.toBeInTheDocument();
  });

  it('renders nothing when user is null (no crash)', () => {
    const { container } = renderWithProviders(<CourierDashboardPage />, {
      user: null,
      queryClient: makeQc(),
    });

    expect(container.firstChild).toBeNull();
    expect(screen.queryByText('Иван Курьеров')).not.toBeInTheDocument();
  });
});

describe('/courier routing', () => {
  it('courier at / redirects to /courier and shows assembled dashboard', async () => {
    server.use(orderDetailHandler);
    renderWithProviders(<AppRoutes />, {
      route: '/',
      user: courierUser,
      queryClient: makeQc(),
    });

    // RoleRedirect sends courier → /courier → CourierDashboardPage renders
    await waitFor(() => {
      expect(screen.getByRole('tab', { name: 'Доступные заказы' })).toBeInTheDocument();
      expect(screen.getByRole('tab', { name: 'Мои доставки' })).toBeInTheDocument();
    });
  });

  it('placeholder copy is absent — CourierDashboardPage replaced CourierPlaceholderPage', async () => {
    server.use(orderDetailHandler);
    renderWithProviders(<AppRoutes />, {
      route: '/',
      user: courierUser,
      queryClient: makeQc(),
    });

    await waitFor(() => {
      expect(screen.getByRole('tab', { name: 'Доступные заказы' })).toBeInTheDocument();
    });
    // The old placeholder copy from CourierPlaceholderPage must not be present
    expect(screen.queryByText(/Скоро здесь будут ваши доставки/i)).not.toBeInTheDocument();
  });

  it('client (role user) at / redirects to /orders — courier change does not regress client routing', async () => {
    renderWithProviders(<AppRoutes />, {
      route: '/',
      user: {
        id: 'u-1',
        first_name: 'К',
        last_name: 'Л',
        role: 'user' as const,
        email: '',
      },
      queryClient: makeQc(),
    });

    // RoleRedirect sends user → /orders → OrdersListPage renders
    await waitFor(() => {
      expect(screen.getByText('Мои заказы')).toBeInTheDocument();
    });
  });
});
