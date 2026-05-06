import { describe, it, expect } from 'vitest';
import { screen, waitFor, fireEvent } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { server } from '@/test/mocks/server';
import { http, HttpResponse } from 'msw';
import { renderWithProviders } from '@/test/test-utils';
import { CourierProfileHeader } from './CourierProfileHeader';

// MSW server lifecycle (listen/reset/close) is handled globally in src/test/setup.ts.
// Per-test override pattern: server.use(...) — reset is automatic in global afterEach.

const mockUser = {
  id: 'crr-1',
  first_name: 'Иван',
  last_name: 'Курьеров',
  role: 'courier' as const,
  email: '',
};

describe('CourierProfileHeader', () => {
  it('renders courier name and rating after MSW resolves', async () => {
    renderWithProviders(
      <CourierProfileHeader courierId="crr-1" initialStatus="offline" />,
      { user: mockUser },
    );

    // Name should be visible immediately from auth context
    expect(screen.getByText('Иван Курьеров')).toBeInTheDocument();

    // Rating loads asynchronously via MSW (averageStars: 4.5, totalRatings: 2)
    await waitFor(() => {
      expect(screen.getByText(/4\.5/)).toBeInTheDocument();
      expect(screen.getByText(/отзывов/)).toBeInTheDocument();
    });
  });

  it('renders empty-state copy when totalRatings === 0', async () => {
    server.use(
      http.get('*/v1/couriers/:id/rating', () =>
        HttpResponse.json({
          averageStars: 0,
          totalRatings: 0,
          ratings: [],
        }),
      ),
    );

    renderWithProviders(
      <CourierProfileHeader courierId="crr-1" initialStatus="offline" />,
      { user: mockUser },
    );

    await waitFor(() => {
      expect(
        screen.getByText('Рейтинг появится после первой доставки'),
      ).toBeInTheDocument();
    });
  });

  it('toggle button shows "Не работаю" when offline, flips to "Работаю" after click', async () => {
    renderWithProviders(
      <CourierProfileHeader courierId="crr-1" initialStatus="offline" />,
      { user: mockUser },
    );

    // Initially offline
    expect(screen.getByRole('button', { name: 'Не работаю' })).toBeInTheDocument();

    // Click the toggle — MSW PATCH returns status:'available'
    fireEvent.click(screen.getByRole('button', { name: 'Не работаю' }));

    // After mutation resolves, button label should flip
    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Работаю' })).toBeInTheDocument();
    });
  });

  // PLSH-06 test cases
  it('PLSH-06-a: shows "Смотреть отзывы" button when totalRatings > 0', async () => {
    // Default MSW returns totalRatings: 2
    renderWithProviders(
      <CourierProfileHeader courierId="crr-1" initialStatus="offline" />,
      { user: mockUser },
    );

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Смотреть отзывы' })).toBeInTheDocument();
    });
  });

  it('PLSH-06-b: hides "Смотреть отзывы" button and shows empty-state when totalRatings === 0', async () => {
    server.use(
      http.get('*/v1/couriers/:id/rating', () =>
        HttpResponse.json({
          averageStars: 0,
          totalRatings: 0,
          ratings: [],
        }),
      ),
    );

    renderWithProviders(
      <CourierProfileHeader courierId="crr-1" initialStatus="offline" />,
      { user: mockUser },
    );

    await waitFor(() => {
      expect(
        screen.getByText('Рейтинг появится после первой доставки'),
      ).toBeInTheDocument();
    });

    expect(screen.queryByRole('button', { name: 'Смотреть отзывы' })).toBeNull();
  });

  it('PLSH-06-c: clicking "Смотреть отзывы" opens modal with ALL entries including empty comment', async () => {
    const user = userEvent.setup();

    server.use(
      http.get('*/v1/couriers/:id/rating', () =>
        HttpResponse.json({
          averageStars: 4,
          totalRatings: 2,
          ratings: [
            {
              deliveryId: 'd1',
              stars: 5,
              comment: 'Отлично',
              createdAt: '2026-05-16T10:00:00Z',
            },
            {
              deliveryId: 'd2',
              stars: 3,
              comment: '',
              createdAt: '2026-05-15T09:00:00Z',
            },
          ],
        }),
      ),
    );

    renderWithProviders(
      <CourierProfileHeader courierId="crr-1" initialStatus="offline" />,
      { user: mockUser },
    );

    // Wait for button to appear
    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Смотреть отзывы' })).toBeInTheDocument();
    });

    // Click to open modal
    await user.click(screen.getByRole('button', { name: 'Смотреть отзывы' }));

    // Radix AlertDialog portal mounts asynchronously (useEffect)
    const dialog = await screen.findByRole('alertdialog');
    expect(dialog).toBeInTheDocument();

    // Both entries should be visible
    expect(screen.getByText(/Отлично/)).toBeInTheDocument();
    expect(screen.getByText('(без комментария)')).toBeInTheDocument();
  });

  it('PLSH-06-d: pressing Escape closes the reviews modal', async () => {
    const user = userEvent.setup();

    renderWithProviders(
      <CourierProfileHeader courierId="crr-1" initialStatus="offline" />,
      { user: mockUser },
    );

    // Wait for button to appear (default MSW totalRatings: 2)
    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Смотреть отзывы' })).toBeInTheDocument();
    });

    // Open modal
    await user.click(screen.getByRole('button', { name: 'Смотреть отзывы' }));
    await screen.findByRole('alertdialog');

    // Press Escape — Radix closes via onOpenChange(false)
    await user.keyboard('{Escape}');

    await waitFor(() => {
      expect(screen.queryByRole('alertdialog')).toBeNull();
    });
  });

  it('PLSH-06-e: header renders no inline review list (<ul>) before opening modal', async () => {
    const { container } = renderWithProviders(
      <CourierProfileHeader courierId="crr-1" initialStatus="offline" />,
      { user: mockUser },
    );

    // Wait for rating data to resolve
    await waitFor(() => {
      expect(screen.getByText(/4\.5/)).toBeInTheDocument();
    });

    // Row-3 inline <ul> must be gone
    expect(container.querySelector('ul')).toBeNull();
  });
});
