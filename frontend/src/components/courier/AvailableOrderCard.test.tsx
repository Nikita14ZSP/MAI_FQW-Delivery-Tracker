import { describe, it, expect, vi } from 'vitest';
import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { renderWithProviders } from '@/test/test-utils';
import { AvailableOrderCard } from './AvailableOrderCard';
import type { AvailableOrder } from '@/lib/api/courier';

const fixture: AvailableOrder = {
  order_id: 'ord-av-1',
  delivery_id: 'dlv-av-1',
  delivery_address: 'Москва, Тверская, 1',
  total_price: 520,
  items_count: 2,
  zone_name: 'Test Zone',
  created_at: '2026-05-16T07:00:00Z',
  items: [
    { name: 'Пицца Маргарита', quantity: 1, price: 390 },
    { name: 'Напиток', quantity: 1, price: 130 },
  ],
};

const emptyItemsFixture: AvailableOrder = { ...fixture, items: [] };

describe('AvailableOrderCard', () => {
  it('renders address, price, items count, zone name', () => {
    renderWithProviders(
      <AvailableOrderCard
        order={fixture}
        onAccept={vi.fn()}
        isAccepting={false}
      />,
    );

    expect(screen.getByText('Москва, Тверская, 1')).toBeInTheDocument();
    expect(screen.getByText('520 ₽')).toBeInTheDocument();
    expect(screen.getByText('2 поз.')).toBeInTheDocument();
    expect(screen.getByText('Test Zone')).toBeInTheDocument();
  });

  it('clicking Принять calls onAccept with delivery_id', async () => {
    const onAccept = vi.fn();
    renderWithProviders(
      <AvailableOrderCard
        order={fixture}
        onAccept={onAccept}
        isAccepting={false}
      />,
    );

    await userEvent.click(screen.getByRole('button', { name: 'Принять' }));
    expect(onAccept).toHaveBeenCalledOnce();
    expect(onAccept).toHaveBeenCalledWith('dlv-av-1');
  });

  it('shows disabled button with "Принимаю…" when isAccepting=true', () => {
    renderWithProviders(
      <AvailableOrderCard
        order={fixture}
        onAccept={vi.fn()}
        isAccepting={true}
      />,
    );

    const btn = screen.getByRole('button', { name: 'Принимаю…' });
    expect(btn).toBeDisabled();
  });

  it('«Подробнее» reveals order composition before accept (CRDR-07)', async () => {
    renderWithProviders(
      <AvailableOrderCard
        order={fixture}
        onAccept={vi.fn()}
        isAccepting={false}
      />,
    );

    // items hidden until expanded
    expect(screen.queryByText(/Пицца Маргарита/)).not.toBeInTheDocument();

    await userEvent.click(
      screen.getByRole('button', { name: 'Подробнее' }),
    );

    expect(screen.getByText(/Пицца Маргарита × 1/)).toBeInTheDocument();
    expect(screen.getByText(/Напиток × 1/)).toBeInTheDocument();
    expect(screen.getByText('390 ₽')).toBeInTheDocument();
    // «Принять» still present alongside «Подробнее» (D-04, same card)
    expect(
      screen.getByRole('button', { name: 'Принять' }),
    ).toBeInTheDocument();
  });

  it('«Подробнее» not rendered when items empty; card + «Принять» still work', async () => {
    const onAccept = vi.fn();
    renderWithProviders(
      <AvailableOrderCard
        order={emptyItemsFixture}
        onAccept={onAccept}
        isAccepting={false}
      />,
    );

    expect(
      screen.queryByRole('button', { name: 'Подробнее' }),
    ).not.toBeInTheDocument();
    expect(screen.getByText('Москва, Тверская, 1')).toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', { name: 'Принять' }));
    expect(onAccept).toHaveBeenCalledOnce();
    expect(onAccept).toHaveBeenCalledWith('dlv-av-1');
  });
});
