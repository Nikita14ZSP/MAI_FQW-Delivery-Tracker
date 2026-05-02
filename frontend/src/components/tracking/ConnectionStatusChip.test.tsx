import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { ConnectionStatusChip } from './ConnectionStatusChip';

describe('ConnectionStatusChip', () => {
  it('renders "Онлайн" with green dot when connected', () => {
    const { container } = render(<ConnectionStatusChip state="connected" />);
    expect(screen.getByText('Онлайн')).toBeInTheDocument();
    expect(container.querySelector('.bg-green-500')).toBeInTheDocument();
  });
  it('renders "Переподключение..." with amber pulsing dot when reconnecting', () => {
    const { container } = render(<ConnectionStatusChip state="reconnecting" />);
    expect(screen.getByText('Переподключение...')).toBeInTheDocument();
    expect(container.querySelector('.bg-amber-500.animate-pulse')).toBeInTheDocument();
  });
  it('renders "Оффлайн" with red dot when offline (after 5 attempts)', () => {
    const { container } = render(<ConnectionStatusChip state="offline" />);
    expect(screen.getByText('Оффлайн')).toBeInTheDocument();
    expect(container.querySelector('.bg-red-500')).toBeInTheDocument();
  });
});
