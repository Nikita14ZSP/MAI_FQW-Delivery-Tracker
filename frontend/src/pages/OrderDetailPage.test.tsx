import { describe, it, expect, afterEach } from 'vitest';
import { screen, waitFor, renderHook } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { Route, Routes } from 'react-router-dom';
import { Toaster } from '@/components/ui/toaster';
import { useToast } from '@/components/ui/use-toast';
import { renderWithProviders } from '@/test/test-utils';
import { server } from '@/test/mocks/server';
import { OrderDetailPage } from './OrderDetailPage';
import type { User } from '@/types/auth';

const BASE = '/v1';

const sampleUser: User = {
  id: 'user-1',
  email: 'a@b.c',
  first_name: 'A',
  last_name: 'B',
  role: 'user',
};

function renderPage() {
  return renderWithProviders(
    <>
      <Routes>
        <Route path="/orders/:id" element={<OrderDetailPage />} />
        <Route path="/orders" element={<div>Orders List</div>} />
      </Routes>
      <Toaster />
    </>,
    { route: '/orders/ord-1', user: sampleUser },
  );
}

describe('OrderDetailPage', () => {
  afterEach(() => {
    localStorage.clear();
  });

  it('renders order header with id-short and StatusBadge', async () => {
    renderPage();
    expect(await screen.findByText('Заказ #ord-1')).toBeInTheDocument();
    // Default sample is created → badge label "Создан"
    expect(screen.getByLabelText('Создан')).toBeInTheDocument();
  });

  it('renders StatusStepper for in_transit order', async () => {
    server.use(
      http.get(`${BASE}/orders/:id`, ({ params }) =>
        HttpResponse.json({
          order: {
            id: params.id,
            userId: 'user-1',
            status: 'ORDER_STATUS_IN_TRANSIT',
            deliveryAddress: 'Москва, ул. Тверская, 1',
            deliveryCoordinates: { latitude: 55.7558, longitude: 37.6173 },
            items: [{ name: 'Пицца', quantity: 1, price: 520 }],
            contactPhone: '+79991234567',
            paymentMethod: 'card_on_delivery',
            createdAt: '2026-05-09T10:00:00Z',
            updatedAt: '2026-05-09T10:00:00Z',
          },
        }),
      ),
    );
    renderPage();
    expect(
      await screen.findByLabelText('В пути: текущий'),
    ).toBeInTheDocument();
  });

  it('renders StatusBanner for cancelled order (no stepper)', async () => {
    server.use(
      http.get(`${BASE}/orders/:id`, ({ params }) =>
        HttpResponse.json({
          order: {
            id: params.id,
            userId: 'user-1',
            status: 'ORDER_STATUS_CANCELLED',
            deliveryAddress: 'Москва, ул. Тверская, 1',
            deliveryCoordinates: { latitude: 55.7558, longitude: 37.6173 },
            items: [{ name: 'Пицца', quantity: 1, price: 520 }],
            contactPhone: '+79991234567',
            paymentMethod: 'card_on_delivery',
            createdAt: '2026-05-09T10:00:00Z',
            updatedAt: '2026-05-09T10:00:00Z',
          },
        }),
      ),
    );
    renderPage();
    expect(await screen.findByText('Заказ отменён')).toBeInTheDocument();
    expect(
      screen.queryByRole('list', { name: 'Прогресс заказа' }),
    ).toBeNull();
  });

  it('renders YELLOW StatusBanner for returned order', async () => {
    server.use(
      http.get(`${BASE}/orders/:id`, ({ params }) =>
        HttpResponse.json({
          order: {
            id: params.id,
            userId: 'user-1',
            status: 'ORDER_STATUS_RETURNED',
            deliveryAddress: 'Москва, ул. Тверская, 1',
            deliveryCoordinates: { latitude: 55.7558, longitude: 37.6173 },
            items: [{ name: 'Пицца', quantity: 1, price: 520 }],
            contactPhone: '+79991234567',
            paymentMethod: 'card_on_delivery',
            createdAt: '2026-05-09T10:00:00Z',
            updatedAt: '2026-05-09T10:00:00Z',
          },
        }),
      ),
    );
    renderPage();
    expect(await screen.findByText('Заказ возвращён')).toBeInTheDocument();
  });

  it('Cancel button visible for created status', async () => {
    renderPage();
    expect(
      await screen.findByRole('button', { name: /Отменить заказ #ord-1/ }),
    ).toBeInTheDocument();
  });

  it('Cancel button NOT visible for delivered status', async () => {
    server.use(
      http.get(`${BASE}/orders/:id`, ({ params }) =>
        HttpResponse.json({
          order: {
            id: params.id,
            userId: 'user-1',
            status: 'ORDER_STATUS_DELIVERED',
            deliveryAddress: 'Москва, ул. Тверская, 1',
            deliveryCoordinates: { latitude: 55.7558, longitude: 37.6173 },
            items: [{ name: 'Пицца', quantity: 1, price: 520 }],
            contactPhone: '+79991234567',
            paymentMethod: 'card_on_delivery',
            createdAt: '2026-05-09T10:00:00Z',
            updatedAt: '2026-05-09T10:00:00Z',
          },
        }),
      ),
    );
    renderPage();
    // Wait for header to render then assert cancel button absent.
    await screen.findByText('Заказ #ord-1');
    expect(
      screen.queryByRole('button', { name: /Отменить заказ #ord-1/ }),
    ).toBeNull();
  });

  it('cancel flow optimistically flips status', async () => {
    const user = userEvent.setup();

    // Override cancel + GET so post-invalidation refetch keeps status=cancelled.
    server.use(
      http.post(`${BASE}/orders/:id/cancel`, ({ params }) =>
        HttpResponse.json({
          order: {
            id: params.id,
            userId: 'user-1',
            status: 'ORDER_STATUS_CANCELLED',
            deliveryAddress: 'Москва, ул. Тверская, 1',
            deliveryCoordinates: { latitude: 55.7558, longitude: 37.6173 },
            items: [{ name: 'Пицца', quantity: 1, price: 520 }],
            contactPhone: '+79991234567',
            paymentMethod: 'card_on_delivery',
            createdAt: '2026-05-09T10:00:00Z',
            updatedAt: '2026-05-09T10:00:00Z',
          },
        }),
      ),
    );

    const { result } = renderHook(() => useToast());
    renderPage();

    await user.click(
      await screen.findByRole('button', { name: /Отменить заказ #ord-1/ }),
    );
    await user.click(screen.getByRole('button', { name: /Да, отменить/ }));

    // Success toast confirms onSuccess fired (optimistic flip happened in onMutate).
    await waitFor(() => {
      expect(
        result.current.toasts.some((t) => t.title === 'Заказ отменён'),
      ).toBe(true);
    });

    // After invalidation refetch returns CANCELLED, the StatusBanner should appear.
    await waitFor(() => {
      expect(screen.queryByText('Заказ отменён')).toBeTruthy();
    });
  });

  it('404 shows "Заказ не найден"', async () => {
    server.use(
      http.get(`${BASE}/orders/:id`, () =>
        HttpResponse.json({ error: 'not found' }, { status: 404 }),
      ),
    );
    renderPage();
    expect(await screen.findByText('Заказ не найден')).toBeInTheDocument();
    expect(
      screen.getByRole('link', { name: '← Мои заказы' }),
    ).toBeInTheDocument();
  });

  it('back link points to /orders', async () => {
    renderPage();
    const link = await screen.findByRole('link', { name: '← Мои заказы' });
    expect(link.getAttribute('href')).toBe('/orders');
  });

  it('renders StatusStepper for delivered order (PLSH-02)', async () => {
    server.use(
      http.get(`${BASE}/orders/:id`, ({ params }) =>
        HttpResponse.json({
          order: {
            id: params.id,
            userId: 'user-1',
            status: 'ORDER_STATUS_DELIVERED',
            deliveryAddress: 'Москва, ул. Тверская, 1',
            deliveryCoordinates: { latitude: 55.7558, longitude: 37.6173 },
            items: [{ name: 'Пицца', quantity: 1, price: 520 }],
            contactPhone: '+79991234567',
            paymentMethod: 'card_on_delivery',
            createdAt: '2026-05-09T10:00:00Z',
            updatedAt: '2026-05-09T10:00:00Z',
          },
        }),
      ),
    );
    renderPage();
    expect(
      await screen.findByLabelText('Доставлен: текущий'),
    ).toBeInTheDocument();
  });

  it('shows RatingCard for delivered order (RATE-04)', async () => {
    server.use(
      http.get(`${BASE}/orders/:id`, ({ params }) =>
        HttpResponse.json({
          order: {
            id: params.id,
            userId: 'user-1',
            delivery_id: 'dlv-mock-1',
            status: 'ORDER_STATUS_DELIVERED',
            deliveryAddress: 'Москва, ул. Тверская, 1',
            deliveryCoordinates: { latitude: 55.7558, longitude: 37.6173 },
            items: [{ name: 'Пицца', quantity: 1, price: 520 }],
            contactPhone: '+79991234567',
            paymentMethod: 'card_on_delivery',
            createdAt: '2026-05-09T10:00:00Z',
            updatedAt: '2026-05-09T10:00:00Z',
          },
        }),
      ),
    );
    renderPage();
    expect(await screen.findByText('Оцените курьера')).toBeInTheDocument();
  });

  it('does NOT show RatingCard for in_transit order (RATE-04)', async () => {
    server.use(
      http.get(`${BASE}/orders/:id`, ({ params }) =>
        HttpResponse.json({
          order: {
            id: params.id,
            userId: 'user-1',
            delivery_id: 'dlv-mock-1',
            status: 'ORDER_STATUS_IN_TRANSIT',
            deliveryAddress: 'Москва, ул. Тверская, 1',
            deliveryCoordinates: { latitude: 55.7558, longitude: 37.6173 },
            items: [{ name: 'Пицца', quantity: 1, price: 520 }],
            contactPhone: '+79991234567',
            paymentMethod: 'card_on_delivery',
            createdAt: '2026-05-09T10:00:00Z',
            updatedAt: '2026-05-09T10:00:00Z',
          },
        }),
      ),
    );
    renderPage();
    expect(await screen.findByText('Заказ #ord-1')).toBeInTheDocument();
    expect(screen.queryByText('Оцените курьера')).not.toBeInTheDocument();
  });

  it('wires real courier name into CourierInfo on delivered order (RATE-04 D-05)', async () => {
    server.use(
      http.get(`${BASE}/orders/:id`, ({ params }) =>
        HttpResponse.json({
          order: {
            id: params.id,
            userId: 'user-1',
            delivery_id: 'dlv-mock-1',
            status: 'ORDER_STATUS_DELIVERED',
            deliveryAddress: 'Москва, ул. Тверская, 1',
            deliveryCoordinates: { latitude: 55.7558, longitude: 37.6173 },
            items: [{ name: 'Пицца', quantity: 1, price: 520 }],
            contactPhone: '+79991234567',
            paymentMethod: 'card_on_delivery',
            createdAt: '2026-05-09T10:00:00Z',
            updatedAt: '2026-05-09T10:00:00Z',
          },
        }),
      ),
    );
    renderPage();
    expect(await screen.findByText('Курьер #crr-mock')).toBeInTheDocument();
  });
});
