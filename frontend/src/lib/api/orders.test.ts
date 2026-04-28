import { describe, it, expect } from 'vitest';
import { listOrders, getOrder, createOrder, cancelOrder } from './orders';

describe('orders API client', () => {
  it('listOrders returns normalized orders', async () => {
    const orders = await listOrders('user-1');
    expect(orders).toHaveLength(1);
    expect(orders[0].status).toBe('created');
    expect(orders[0].id).toBe('ord-1');
  });

  it('getOrder returns normalized order', async () => {
    const order = await getOrder('ord-42');
    expect(order.id).toBe('ord-42');
    expect(order.status).toBe('created');
  });

  it('createOrder posts and returns normalized order with new id', async () => {
    const order = await createOrder({
      user_id: 'user-1',
      delivery_address: 'Москва, Тверская, 1',
      delivery_coordinates: { latitude: 55.7558, longitude: 37.6173 },
      items: [{ name: 'Пицца', quantity: 1, price: 520 }],
      contact_phone: '+79991234567',
      payment_method: 'card_on_delivery',
    });
    expect(order.id).toBe('ord-new');
    expect(order.status).toBe('created');
  });

  it('cancelOrder hits cancel endpoint and returns normalized order with status cancelled', async () => {
    const order = await cancelOrder('ord-7');
    expect(order.id).toBe('ord-7');
    expect(order.status).toBe('cancelled');
  });
});
