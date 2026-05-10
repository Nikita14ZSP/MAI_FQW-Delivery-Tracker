import { describe, it, expect, afterEach } from 'vitest';
import { screen, waitFor, renderHook } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Toaster } from '@/components/ui/toaster';
import { useToast } from '@/components/ui/use-toast';
import { renderWithProviders } from '@/test/test-utils';
import { RatingCard } from './RatingCard';

afterEach(() => {
  localStorage.clear();
});

function renderCard() {
  return renderWithProviders(
    <>
      <RatingCard orderId="ord-1" deliveryId="dlv-mock-1" />
      <Toaster />
    </>,
  );
}

describe('RatingCard', () => {
  it('renders 5 star buttons; submit disabled at 0 stars, enabled after picking 4', async () => {
    const user = userEvent.setup();
    renderCard();

    const stars = screen.getAllByRole('button', { name: /Оценить на \d звёзд/ });
    expect(stars).toHaveLength(5);

    const submit = screen.getByRole('button', { name: 'Отправить оценку' });
    expect(submit).toBeDisabled();

    await user.click(
      screen.getByRole('button', { name: 'Оценить на 4 звёзд' }),
    );
    expect(submit).toBeEnabled();
  });

  it('submit with 4 stars + comment replaces form with read-only block and dispatches a toast', async () => {
    const user = userEvent.setup();
    const { result } = renderHook(() => useToast());
    renderCard();

    await user.click(
      screen.getByRole('button', { name: 'Оценить на 4 звёзд' }),
    );
    await user.type(
      screen.getByRole('textbox'),
      'Хорошо',
    );
    await user.click(screen.getByRole('button', { name: 'Отправить оценку' }));

    expect(await screen.findByText('Вы оценили курьера')).toBeInTheDocument();
    expect(screen.getByText('Хорошо')).toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: 'Отправить оценку' }),
    ).not.toBeInTheDocument();
    await waitFor(() =>
      expect(result.current.toasts[0]?.title).toBe('Спасибо за оценку'),
    );
  });

  it('persists the submitted rating to localStorage under rating:submitted:ord-1', async () => {
    const user = userEvent.setup();
    renderCard();

    await user.click(
      screen.getByRole('button', { name: 'Оценить на 4 звёзд' }),
    );
    await user.type(screen.getByRole('textbox'), 'Хорошо');
    await user.click(screen.getByRole('button', { name: 'Отправить оценку' }));

    await waitFor(() => {
      const raw = localStorage.getItem('rating:submitted:ord-1');
      expect(raw).toBeTruthy();
      expect(JSON.parse(raw as string)).toEqual({ stars: 4, comment: 'Хорошо' });
    });
  });

  it('skip sets rating:skipped:ord-1 flag and hides the form', async () => {
    const user = userEvent.setup();
    renderCard();

    await user.click(screen.getByRole('button', { name: 'Пропустить' }));

    expect(localStorage.getItem('rating:skipped:ord-1')).toBe('1');
    expect(
      screen.queryByRole('button', { name: 'Отправить оценку' }),
    ).not.toBeInTheDocument();
  });

  it('renders the read-only block immediately when a submitted rating is already in localStorage', async () => {
    localStorage.setItem(
      'rating:submitted:ord-1',
      JSON.stringify({ stars: 5, comment: 'было' }),
    );
    renderCard();

    expect(await screen.findByText('Вы оценили курьера')).toBeInTheDocument();
    expect(screen.getByText('было')).toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: 'Отправить оценку' }),
    ).not.toBeInTheDocument();
  });
});
