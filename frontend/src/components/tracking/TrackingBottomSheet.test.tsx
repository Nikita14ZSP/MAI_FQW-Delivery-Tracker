import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { TrackingBottomSheet } from './TrackingBottomSheet';

describe('TrackingBottomSheet', () => {
  it('renders children expanded by default', () => {
    render(
      <TrackingBottomSheet>
        <div>Content here</div>
      </TrackingBottomSheet>,
    );
    expect(screen.getByText('Content here')).toBeInTheDocument();
    expect(screen.getByRole('button')).toHaveAttribute('aria-expanded', 'true');
  });

  it('toggles state on button click and calls onToggle callback', () => {
    const onToggle = vi.fn();
    render(
      <TrackingBottomSheet onToggle={onToggle}>
        <div>Content</div>
      </TrackingBottomSheet>,
    );
    fireEvent.click(screen.getByRole('button'));
    expect(onToggle).toHaveBeenCalledWith(false);
    expect(screen.getByRole('button')).toHaveAttribute('aria-expanded', 'false');
    fireEvent.click(screen.getByRole('button'));
    expect(onToggle).toHaveBeenCalledWith(true);
  });

  it('respects defaultExpanded=false', () => {
    render(
      <TrackingBottomSheet defaultExpanded={false}>
        <div>Hidden initially</div>
      </TrackingBottomSheet>,
    );
    expect(screen.getByRole('button')).toHaveAttribute('aria-expanded', 'false');
  });
});
