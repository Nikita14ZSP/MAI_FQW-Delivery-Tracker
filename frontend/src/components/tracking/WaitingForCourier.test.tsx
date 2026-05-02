import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { WaitingForCourier } from './WaitingForCourier';

describe('WaitingForCourier', () => {
  it('renders loader icon and "Поиск курьера..." text', () => {
    const { container } = render(<WaitingForCourier />);
    expect(screen.getByText('Поиск курьера...')).toBeInTheDocument();
    expect(container.querySelector('.animate-spin')).toBeInTheDocument();
  });
});
