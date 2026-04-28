import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { PaymentMethodRadio } from './PaymentMethodRadio';

describe('PaymentMethodRadio', () => {
  it('renders all 3 payment options', () => {
    render(<PaymentMethodRadio value="card_on_delivery" onChange={() => {}} />);
    expect(screen.getByText('Картой курьеру')).toBeInTheDocument();
    expect(screen.getByText('Наличными курьеру')).toBeInTheDocument();
    expect(screen.getByText('Онлайн оплата')).toBeInTheDocument();
  });

  it('online option hint is exactly "Тестовый режим" (no mock)', () => {
    render(<PaymentMethodRadio value="online" onChange={() => {}} />);
    expect(screen.getByText('Тестовый режим')).toBeInTheDocument();
  });

  it('does not contain the word "mock" anywhere', () => {
    render(<PaymentMethodRadio value="online" onChange={() => {}} />);
    expect(screen.queryByText(/mock/i)).toBeNull();
    expect(screen.queryByText(/\(mock\)/)).toBeNull();
  });
});
