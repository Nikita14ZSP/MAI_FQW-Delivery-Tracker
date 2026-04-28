import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { StatusBanner } from './StatusBanner';

describe('StatusBanner', () => {
  it('cancelled renders red banner with title "Заказ отменён"', () => {
    render(<StatusBanner status="cancelled" />);
    expect(screen.getByText('Заказ отменён')).toBeInTheDocument();
    const title = screen.getByText('Заказ отменён');
    expect(title.className).toContain('text-red-700');
  });

  it('failed renders red banner with title "Доставка не состоялась"', () => {
    render(<StatusBanner status="failed" />);
    expect(screen.getByText('Доставка не состоялась')).toBeInTheDocument();
    const title = screen.getByText('Доставка не состоялась');
    expect(title.className).toContain('text-red-700');
  });

  it('returned renders YELLOW banner (UI-SPEC contract)', () => {
    render(<StatusBanner status="returned" />);
    const title = screen.getByText('Заказ возвращён');
    expect(title).toBeInTheDocument();
    expect(title.className).toContain('text-yellow-800');
    const banner = screen.getByRole('status');
    expect(banner.className).toContain('bg-yellow-50');
    expect(banner.className).toContain('border-yellow-200');
  });
});
