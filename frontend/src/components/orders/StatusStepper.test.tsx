import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { StatusStepper } from './StatusStepper';

describe('StatusStepper', () => {
  it('renders 6 step labels for in_transit', () => {
    render(<StatusStepper currentStatus="in_transit" />);
    expect(screen.getByText('Создан')).toBeInTheDocument();
    expect(screen.getByText('Подтверждён')).toBeInTheDocument();
    expect(screen.getByText('Назначен курьеру')).toBeInTheDocument();
    expect(screen.getByText('Забран')).toBeInTheDocument();
    expect(screen.getByText('В пути')).toBeInTheDocument();
    expect(screen.getByText('Доставлен')).toBeInTheDocument();
  });

  it('step "created" is past for current status "in_transit"', () => {
    render(<StatusStepper currentStatus="in_transit" />);
    expect(screen.getByLabelText('Создан: пройден')).toBeInTheDocument();
  });

  it('step "in_transit" is current', () => {
    render(<StatusStepper currentStatus="in_transit" />);
    expect(screen.getByLabelText('В пути: текущий')).toBeInTheDocument();
  });

  it('step "delivered" is future for in_transit', () => {
    render(<StatusStepper currentStatus="in_transit" />);
    expect(screen.getByLabelText('Доставлен: предстоит')).toBeInTheDocument();
  });

  it('renders nothing for cancelled status', () => {
    const { container } = render(
      <StatusStepper currentStatus={'cancelled' as never} />,
    );
    expect(container.firstChild).toBeNull();
  });

  it('current step circle has animate-pulse class', () => {
    render(<StatusStepper currentStatus="in_transit" />);
    const currentItem = screen.getByLabelText('В пути: текущий');
    const circle = currentItem.querySelector('div');
    expect(circle?.className).toContain('animate-pulse');
  });

  it('renders delivered as the current step with all prior steps complete (PLSH-02)', () => {
    render(<StatusStepper currentStatus="delivered" />);
    // Delivered step should be marked as current
    expect(screen.getByLabelText('Доставлен: текущий')).toBeInTheDocument();
    // The current step circle has animate-pulse
    const deliveredItem = screen.getByLabelText('Доставлен: текущий');
    const circle = deliveredItem.querySelector('div');
    expect(circle?.className).toContain('animate-pulse');
    // All prior steps (e.g. Создан) should be marked as past (пройден)
    expect(screen.getByLabelText('Создан: пройден')).toBeInTheDocument();
  });

  it('renders all 6 labels for delivered status (PLSH-05)', () => {
    render(<StatusStepper currentStatus="delivered" />);
    for (const label of [
      'Создан',
      'Подтверждён',
      'Назначен курьеру',
      'Забран',
      'В пути',
      'Доставлен',
    ]) {
      expect(screen.getByText(label)).toBeInTheDocument();
    }
  });
});
