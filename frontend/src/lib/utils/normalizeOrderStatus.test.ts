import { describe, it, expect } from 'vitest';
import {
  normalizeOrderStatusString,
  normalizeOrder,
} from './normalizeOrderStatus';
import type { OrderRaw } from '@/types/order';

describe('normalizeOrderStatusString', () => {
  it('strips ORDER_STATUS_ prefix and lowercases CREATED', () => {
    expect(normalizeOrderStatusString('ORDER_STATUS_CREATED')).toBe('created');
  });

  it('strips ORDER_STATUS_ prefix and lowercases IN_TRANSIT', () => {
    expect(normalizeOrderStatusString('ORDER_STATUS_IN_TRANSIT')).toBe(
      'in_transit',
    );
  });

  it('strips ORDER_STATUS_ prefix and lowercases RETURNED', () => {
    expect(normalizeOrderStatusString('ORDER_STATUS_RETURNED')).toBe(
      'returned',
    );
  });

  it('is idempotent for already-normalized status', () => {
    expect(normalizeOrderStatusString('created')).toBe('created');
  });

  it('returns "created" default for undefined', () => {
    expect(normalizeOrderStatusString(undefined)).toBe('created');
  });

  it('returns "created" default for null', () => {
    expect(normalizeOrderStatusString(null)).toBe('created');
  });
});

describe('normalizeOrder', () => {
  it('normalizes status while preserving other fields', () => {
    const raw: OrderRaw = {
      id: 'ord-1',
      userId: 'user-1',
      status: 'ORDER_STATUS_CANCELLED',
      deliveryAddress: 'Москва, Тверская, 1',
      deliveryCoordinates: { latitude: 55.7558, longitude: 37.6173 },
      items: [{ name: 'Пицца', quantity: 1, price: 520 }],
      contactPhone: '+79991234567',
      paymentMethod: 'card_on_delivery',
      createdAt: '2026-05-09T10:00:00Z',
      updatedAt: '2026-05-09T10:00:00Z',
    };
    const out = normalizeOrder(raw);
    expect(out.status).toBe('cancelled');
    expect(out.id).toBe('ord-1');
    expect(out.deliveryAddress).toBe('Москва, Тверская, 1');
    expect(out.items).toEqual(raw.items);
  });
});
