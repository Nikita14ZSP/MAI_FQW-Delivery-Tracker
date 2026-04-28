import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QtyStepper } from './QtyStepper';

describe('QtyStepper', () => {
  it('renders quantity 0 with decrement disabled', () => {
    render(
      <QtyStepper value={0} onIncrement={vi.fn()} onDecrement={vi.fn()} />,
    );
    expect(screen.getByText('0')).toBeInTheDocument();
    expect(screen.getByLabelText('Уменьшить количество')).toBeDisabled();
  });

  it('clicking + fires onIncrement', async () => {
    const user = userEvent.setup();
    const onIncrement = vi.fn();
    render(
      <QtyStepper
        value={0}
        onIncrement={onIncrement}
        onDecrement={vi.fn()}
      />,
    );
    await user.click(screen.getByLabelText('Увеличить количество'));
    expect(onIncrement).toHaveBeenCalledTimes(1);
  });

  it('clicking - fires onDecrement when value > 0', async () => {
    const user = userEvent.setup();
    const onDecrement = vi.fn();
    render(
      <QtyStepper
        value={2}
        onIncrement={vi.fn()}
        onDecrement={onDecrement}
      />,
    );
    await user.click(screen.getByLabelText('Уменьшить количество'));
    expect(onDecrement).toHaveBeenCalledTimes(1);
  });

  it('decrement is disabled when value is 0', () => {
    render(
      <QtyStepper value={0} onIncrement={vi.fn()} onDecrement={vi.fn()} />,
    );
    expect(screen.getByLabelText('Уменьшить количество')).toHaveAttribute(
      'disabled',
    );
  });
});
