import { describe, it, expect } from 'vitest';
import { CreateOrderSchema, OrderItemSchema } from './order';

describe('CreateOrderSchema', () => {
  const validPayload = {
    delivery_address: 'Москва, Тверская, 1',
    delivery_lat: 55.7558,
    delivery_lng: 37.6173,
    items: [{ name: 'Пицца', quantity: 1, price: 520 }],
    contact_phone: '+79991234567',
    payment_method: 'card_on_delivery' as const,
  };

  it('accepts valid payload', () => {
    const result = CreateOrderSchema.safeParse(validPayload);
    expect(result.success).toBe(true);
  });

  it('rejects empty items', () => {
    const result = CreateOrderSchema.safeParse({ ...validPayload, items: [] });
    expect(result.success).toBe(false);
    if (!result.success) {
      const msgs = result.error.issues.map((i) => i.message);
      expect(msgs).toContain('Добавьте хотя бы одно блюдо');
    }
  });

  it('rejects missing delivery_lat', () => {
    const { delivery_lat: _omit, ...rest } = validPayload;
    void _omit;
    const result = CreateOrderSchema.safeParse(rest);
    expect(result.success).toBe(false);
  });

  it('rejects invalid phone', () => {
    const result = CreateOrderSchema.safeParse({
      ...validPayload,
      contact_phone: '8-999-123-45-67',
    });
    expect(result.success).toBe(false);
    if (!result.success) {
      const msgs = result.error.issues.map((i) => i.message);
      expect(msgs).toContain('Введите телефон в формате +7XXXXXXXXXX');
    }
  });

  it('rejects unknown payment_method', () => {
    const result = CreateOrderSchema.safeParse({
      ...validPayload,
      payment_method: 'bitcoin',
    });
    expect(result.success).toBe(false);
  });

  it('normalizes a space+dash formatted phone to +7XXXXXXXXXX', () => {
    const result = CreateOrderSchema.safeParse({
      ...validPayload,
      contact_phone: '+7 999 123-45-67',
    });
    expect(result.success).toBe(true);
    if (result.success) {
      expect(result.data.contact_phone).toBe('+79991234567');
    }
  });

  it('normalizes a phone with parentheses and dots', () => {
    const result = CreateOrderSchema.safeParse({
      ...validPayload,
      contact_phone: '+7 (999) 123.45.67',
    });
    expect(result.success).toBe(true);
    if (result.success) {
      expect(result.data.contact_phone).toBe('+79991234567');
    }
  });
});

describe('OrderItemSchema', () => {
  it('rejects negative price', () => {
    const result = OrderItemSchema.safeParse({
      name: 'Пицца',
      quantity: 1,
      price: -10,
    });
    expect(result.success).toBe(false);
  });

  it('rejects zero quantity', () => {
    const result = OrderItemSchema.safeParse({
      name: 'Пицца',
      quantity: 0,
      price: 100,
    });
    expect(result.success).toBe(false);
  });
});
