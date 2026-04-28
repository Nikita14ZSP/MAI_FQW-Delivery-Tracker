import { describe, it, expect, vi, beforeEach } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { QueryClient } from '@tanstack/react-query';
import { renderWithProviders } from '@/test/test-utils';
import { server } from '@/test/mocks/server';
import type { User } from '@/types/auth';

const navigateMock = vi.fn();
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>(
    'react-router-dom',
  );
  return {
    ...actual,
    useNavigate: () => navigateMock,
  };
});

import { OrdersListPage } from './OrdersListPage';

const mockUser: User = {
  id: 'user-1',
  email: 'tester@example.com',
  first_name: 'Test',
  last_name: 'User',
  role: 'user',
};

function makeOrderRaw(
  id: string,
  status:
    | 'ORDER_STATUS_CREATED'
    | 'ORDER_STATUS_DELIVERED'
    | 'ORDER_STATUS_CANCELLED' = 'ORDER_STATUS_CREATED',
) {
  return {
    id,
    userId: 'user-1',
    status,
    deliveryAddress: 'Москва, ул. Тверская, 1',
    deliveryCoordinates: { latitude: 55.7558, longitude: 37.6173 },
    items: [{ name: 'Пицца Маргарита', quantity: 1, price: 520 }],
    contactPhone: '+79991234567',
    paymentMethod: 'card_on_delivery',
    createdAt: '2026-05-09T10:00:00Z',
    updatedAt: '2026-05-09T10:00:00Z',
  };
}

describe('OrdersListPage', () => {
  beforeEach(() => {
    navigateMock.mockReset();
  });

  it('renders heading "Мои заказы"', async () => {
    renderWithProviders(<OrdersListPage />, {
      route: '/orders',
      user: mockUser,
    });
    expect(
      await screen.findByRole('heading', { name: 'Мои заказы' }),
    ).toBeInTheDocument();
  });

  it('shows skeleton cards while loading', () => {
    renderWithProviders(<OrdersListPage />, {
      route: '/orders',
      user: mockUser,
    });
    // 3 skeleton cards rendered initially (each contains animate-pulse Skeletons)
    const container = document.querySelector('[aria-busy="true"]');
    expect(container).not.toBeNull();
    const skeletons = container?.querySelectorAll('.animate-pulse') ?? [];
    expect(skeletons.length).toBeGreaterThanOrEqual(3);
  });

  it('renders OrderCard after fetch resolves (badge label "Создан")', async () => {
    renderWithProviders(<OrdersListPage />, {
      route: '/orders',
      user: mockUser,
    });
    expect(await screen.findByText('Создан')).toBeInTheDocument();
  });

  it('clicking RefreshButton triggers invalidateQueries on the orders query key', async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    const spy = vi.spyOn(queryClient, 'invalidateQueries');
    renderWithProviders(<OrdersListPage />, {
      route: '/orders',
      user: mockUser,
      queryClient,
    });
    await screen.findByText('Создан');
    const btn = screen.getByLabelText('Обновить список заказов');
    await userEvent.setup().click(btn);
    expect(spy).toHaveBeenCalled();
    const args = spy.mock.calls[0]?.[0] as { queryKey?: unknown };
    expect(args.queryKey).toEqual(['orders', 'user-1']);
  });

  it('switching to Завершённые tab shows EmptyOrders completed variant when no completed orders', async () => {
    // default MSW handler returns 1 created order — no completed
    renderWithProviders(<OrdersListPage />, {
      route: '/orders',
      user: mockUser,
    });
    await screen.findByText('Создан');
    const completedTab = screen.getByRole('tab', { name: 'Завершённые' });
    await userEvent.setup().click(completedTab);
    expect(
      await screen.findByText('Нет завершённых заказов'),
    ).toBeInTheDocument();
    expect(screen.queryByText('Создан')).not.toBeInTheDocument();
  });

  it('URL ?status=completed defaults the active tab to Завершённые', async () => {
    renderWithProviders(<OrdersListPage />, {
      route: '/orders?status=completed',
      user: mockUser,
    });
    await screen.findByRole('heading', { name: 'Мои заказы' });
    const completedTab = screen.getByRole('tab', { name: 'Завершённые' });
    await waitFor(() => {
      expect(completedTab.getAttribute('data-state')).toBe('active');
    });
  });

  it('clicking an OrderCard navigates to /orders/:id', async () => {
    renderWithProviders(<OrdersListPage />, {
      route: '/orders',
      user: mockUser,
    });
    await screen.findByText('Создан');
    const card = screen.getByRole('button', { name: /Создан/ });
    await userEvent.setup().click(card);
    expect(navigateMock).toHaveBeenCalledWith('/orders/ord-1');
  });

  it('empty active list shows ShoppingBag empty state with CTA', async () => {
    server.use(
      http.get('/v1/orders', () =>
        HttpResponse.json({
          orders: [],
          pagination: { total: 0, page: 1, pageSize: 100 },
        }),
      ),
    );
    renderWithProviders(<OrdersListPage />, {
      route: '/orders',
      user: mockUser,
    });
    expect(
      await screen.findByText('У вас пока нет заказов'),
    ).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /Создать заказ/ })).toHaveAttribute(
      'href',
      '/orders/new',
    );
  });

  it('error state shows retry RefreshButton when listOrders fails', async () => {
    server.use(
      http.get('/v1/orders', () =>
        HttpResponse.json({ message: 'boom' }, { status: 500 }),
      ),
    );
    renderWithProviders(<OrdersListPage />, {
      route: '/orders',
      user: mockUser,
    });
    expect(
      await screen.findByText('Не удалось загрузить заказы'),
    ).toBeInTheDocument();
    expect(
      screen.getAllByLabelText('Обновить список заказов').length,
    ).toBeGreaterThanOrEqual(1);
  });

  it('"Все" tab shows orders of all statuses', async () => {
    server.use(
      http.get('/v1/orders', () =>
        HttpResponse.json({
          orders: [
            makeOrderRaw('ord-active', 'ORDER_STATUS_CREATED'),
            makeOrderRaw('ord-done', 'ORDER_STATUS_DELIVERED'),
          ],
          pagination: { total: 2, page: 1, pageSize: 100 },
        }),
      ),
    );
    renderWithProviders(<OrdersListPage />, {
      route: '/orders?status=all',
      user: mockUser,
    });
    expect(await screen.findByText('Создан')).toBeInTheDocument();
    expect(await screen.findByText('Доставлен')).toBeInTheDocument();
  });
});
