import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, act } from '@testing-library/react';
import { EtaChip } from './EtaChip';

beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(new Date('2026-05-14T12:00:00Z'));
});
afterEach(() => vi.useRealTimers());

describe('EtaChip', () => {
  it('renders "~N мин" diff from RFC3339', () => {
    render(<EtaChip etaIso="2026-05-14T12:12:00Z" />);
    expect(screen.getByText('~12 мин')).toBeInTheDocument();
  });

  it('renders "Прибыл" when eta in past or now', () => {
    render(<EtaChip etaIso="2026-05-14T11:59:00Z" />);
    expect(screen.getByText('Прибыл')).toBeInTheDocument();
  });

  it('returns null when etaIso is null', () => {
    const { container } = render(<EtaChip etaIso={null} />);
    expect(container.firstChild).toBeNull();
  });

  it('re-renders every minute via interval (text counts down: ~12 → ~11 → ~10)', () => {
    // SystemTime starts at 12:00:00, eta at 12:12:00 → initial diff = 12 minutes.
    // vi.useFakeTimers() also mocks Date — so advanceTimersByTime advances Date.now() too.
    const eta = '2026-05-14T12:12:00Z';

    render(<EtaChip etaIso={eta} />);
    expect(screen.getByText('~12 мин')).toBeInTheDocument();

    // Tick 1: advance fake timers +1 min → interval fires, Date.now()=12:01:00, diff=11
    act(() => {
      vi.advanceTimersByTime(60_000);
    });
    expect(screen.getByText('~11 мин')).toBeInTheDocument();

    // Tick 2: advance another minute — Date.now()=12:02:00, diff=10
    act(() => {
      vi.advanceTimersByTime(60_000);
    });
    expect(screen.getByText('~10 мин')).toBeInTheDocument();
  });
});
