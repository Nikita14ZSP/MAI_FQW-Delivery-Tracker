import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { StatusBadge } from './StatusBadge';
import {
  ORDER_STATUS_LABELS,
  type OrderStatus,
} from '@/lib/constants';

const ALL_STATUSES: OrderStatus[] = [
  'created',
  'confirmed',
  'assigned',
  'picked_up',
  'in_transit',
  'delivered',
  'cancelled',
  'failed',
  'returned',
];

describe('StatusBadge', () => {
  it('renders Russian label for every status', () => {
    for (const status of ALL_STATUSES) {
      const { unmount } = render(<StatusBadge status={status} />);
      const label = ORDER_STATUS_LABELS[status];
      expect(screen.getByText(label)).toBeInTheDocument();
      unmount();
    }
  });

  it('applies yellow classes for returned status (UI-SPEC contract)', () => {
    // Contract: returned must use 'bg-yellow-50 text-yellow-800 border-yellow-200'
    render(<StatusBadge status="returned" />);
    const el = screen.getByLabelText('Возвращён');
    expect(el.className).toContain('bg-yellow-50');
    expect(el.className).toContain('text-yellow-800');
    expect(el.className).toContain('border-yellow-200');
  });

  it('applies red classes for cancelled', () => {
    render(<StatusBadge status="cancelled" />);
    const el = screen.getByLabelText('Отменён');
    expect(el.className).toContain('bg-red-50');
    expect(el.className).toContain('text-red-700');
    expect(el.className).toContain('border-red-200');
  });

  it('applies red classes for failed', () => {
    render(<StatusBadge status="failed" />);
    const el = screen.getByLabelText('Сбой доставки');
    expect(el.className).toContain('bg-red-50');
    expect(el.className).toContain('text-red-700');
    expect(el.className).toContain('border-red-200');
  });

  it('applies green classes for delivered', () => {
    render(<StatusBadge status="delivered" />);
    const el = screen.getByLabelText('Доставлен');
    expect(el.className).toContain('bg-green-50');
    expect(el.className).toContain('text-green-700');
    expect(el.className).toContain('border-green-200');
  });

  it('aria-label is the Russian label', () => {
    render(<StatusBadge status="delivered" />);
    expect(screen.getByLabelText('Доставлен')).toBeInTheDocument();
  });
});
