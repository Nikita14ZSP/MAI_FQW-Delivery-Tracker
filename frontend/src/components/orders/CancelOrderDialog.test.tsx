import { describe, it, expect, beforeEach } from 'vitest';
import { screen, waitFor, renderHook, act } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse, delay } from 'msw';
import { QueryClient } from '@tanstack/react-query';
import { Toaster } from '@/components/ui/toaster';
import { renderWithProviders } from '@/test/test-utils';
import { server } from '@/test/mocks/server';
import { useToast } from '@/components/ui/use-toast';
import { CancelOrderDialog } from './CancelOrderDialog';
import type { Order } from '@/types/order';

const BASE = '/v1';

function makeQc() {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
}

const seedOrder: Order = {
  id: 'ord-1',
  userId: 'user-1',
  status: 'created',
  deliveryAddress: 'Москва, ул. Тверская, 1',
  deliveryCoordinates: { latitude: 55.7558, longitude: 37.6173 },
  items: [{ name: 'Пицца Маргарита', quantity: 1, price: 520 }],
  contactPhone: '+79991234567',
  paymentMethod: 'card_on_delivery',
  createdAt: '2026-05-09T10:00:00Z',
  updatedAt: '2026-05-09T10:00:00Z',
};

function seed(qc: QueryClient) {
  qc.setQueryData<Order[]>(['orders', 'user-1'], [seedOrder]);
  qc.setQueryData<Order>(['order', 'ord-1'], seedOrder);
}

describe('CancelOrderDialog', () => {
  beforeEach(async () => {
    // Toast module-scope memoryState persists across tests (TOAST_LIMIT=1).
    // Reset by mounting hook, dispatching ADD_TOAST that gets immediately dismissed.
    // Subsequent toasts in this test will replace the marker due to slice(0, 1).
    const { result, unmount } = renderHook(() => useToast());
    await act(async () => {
      // Dismiss everything currently in the queue.
      result.current.dismiss();
    });
    unmount();
  });

  it('clicking trigger opens dialog with title "Отменить заказ?"', async () => {
    const user = userEvent.setup();
    const qc = makeQc();
    seed(qc);
    renderWithProviders(
      <>
        <CancelOrderDialog orderId="ord-1" userId="user-1" />
        <Toaster />
      </>,
      { queryClient: qc },
    );

    await user.click(
      screen.getByRole('button', { name: /Отменить заказ #ord-1/ }),
    );
    expect(await screen.findByText('Отменить заказ?')).toBeInTheDocument();
  });

  it('confirm flips list + detail status to cancelled optimistically', async () => {
    const user = userEvent.setup();
    const qc = makeQc();
    seed(qc);

    // Delay server so we can observe optimistic state
    server.use(
      http.post(`${BASE}/orders/:id/cancel`, async ({ params }) => {
        await delay(200);
        return HttpResponse.json({
          order: {
            id: params.id,
            userId: 'user-1',
            status: 'ORDER_STATUS_CANCELLED',
            deliveryAddress: seedOrder.deliveryAddress,
            deliveryCoordinates: seedOrder.deliveryCoordinates,
            items: seedOrder.items,
            contactPhone: seedOrder.contactPhone,
            paymentMethod: seedOrder.paymentMethod,
            createdAt: seedOrder.createdAt,
            updatedAt: seedOrder.updatedAt,
          },
        });
      }),
    );

    renderWithProviders(
      <>
        <CancelOrderDialog orderId="ord-1" userId="user-1" />
        <Toaster />
      </>,
      { queryClient: qc },
    );

    await user.click(
      screen.getByRole('button', { name: /Отменить заказ #ord-1/ }),
    );
    await user.click(screen.getByRole('button', { name: /Да, отменить/ }));

    // After microtask, optimistic state must be applied
    await waitFor(() => {
      const detail = qc.getQueryData<Order>(['order', 'ord-1']);
      expect(detail?.status).toBe('cancelled');
    });
    const list = qc.getQueryData<Order[]>(['orders', 'user-1']);
    expect(list?.[0].status).toBe('cancelled');
  });

  it('success path shows "Заказ отменён" toast', async () => {
    const user = userEvent.setup();
    const qc = makeQc();
    seed(qc);

    renderWithProviders(
      <>
        <CancelOrderDialog orderId="ord-1" userId="user-1" />
        <Toaster />
      </>,
      { queryClient: qc },
    );

    await user.click(
      screen.getByRole('button', { name: /Отменить заказ #ord-1/ }),
    );
    await user.click(screen.getByRole('button', { name: /Да, отменить/ }));

    const { result } = renderHook(() => useToast());
    await waitFor(() => {
      expect(
        result.current.toasts.some((t) => t.title === 'Заказ отменён'),
      ).toBe(true);
    });
  });

  it('500 error rolls back to "created" and shows generic error toast', async () => {
    const user = userEvent.setup();
    const qc = makeQc();
    seed(qc);

    server.use(
      http.post(`${BASE}/orders/:id/cancel`, async () => {
        return HttpResponse.json({ error: 'oops' }, { status: 500 });
      }),
    );

    renderWithProviders(
      <>
        <CancelOrderDialog orderId="ord-1" userId="user-1" />
        <Toaster />
      </>,
      { queryClient: qc },
    );

    await user.click(
      screen.getByRole('button', { name: /Отменить заказ #ord-1/ }),
    );
    await user.click(screen.getByRole('button', { name: /Да, отменить/ }));

    // Verify toast dispatched via the hook (avoids stale DOM coupling).
    const { result } = renderHook(() => useToast());
    await waitFor(() => {
      expect(
        result.current.toasts.some(
          (t) => t.title === 'Ошибка при отмене заказа',
        ),
      ).toBe(true);
    });

    await waitFor(() => {
      const detail = qc.getQueryData<Order>(['order', 'ord-1']);
      expect(detail?.status).toBe('created');
    });
  });

  it('409 error rolls back and shows specific 409 toast', async () => {
    const user = userEvent.setup();
    const qc = makeQc();
    seed(qc);

    server.use(
      http.post(`${BASE}/orders/:id/cancel`, async () => {
        return HttpResponse.json({ error: 'conflict' }, { status: 409 });
      }),
    );

    // Mount the toast hook FIRST so we have a live snapshot subscribing to dispatches.
    const { result } = renderHook(() => useToast());

    renderWithProviders(
      <>
        <CancelOrderDialog orderId="ord-1" userId="user-1" />
        <Toaster />
      </>,
      { queryClient: qc },
    );

    await user.click(
      screen.getByRole('button', { name: /Отменить заказ #ord-1/ }),
    );
    await user.click(screen.getByRole('button', { name: /Да, отменить/ }));

    await waitFor(() => {
      expect(
        result.current.toasts.some(
          (t) => t.title === 'Заказ уже принят курьером, отмена невозможна',
        ),
      ).toBe(true);
    });

    await waitFor(() => {
      const detail = qc.getQueryData<Order>(['order', 'ord-1']);
      expect(detail?.status).toBe('created');
    });
  });
});
