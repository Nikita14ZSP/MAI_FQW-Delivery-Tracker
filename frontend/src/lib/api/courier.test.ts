import { describe, it, expect } from 'vitest';
import { server } from '@/test/mocks/server';
import { http, HttpResponse } from 'msw';
import {
  normalizeCourierRating,
  normalizeAvailableOrder,
  normalizeCourierStatus,
  normalizeCourierProfile,
  getCourierRating,
  getCourierProfile,
  updateCourierStatus,
  listAvailableOrders,
  getCourierDeliveries,
  updateOrderStatus,
  postCourierLocation,
} from './courier';

// MSW server lifecycle (listen/reset/close) is handled globally in src/test/setup.ts.
// Per-test override pattern: server.use(...) — reset is automatic in global afterEach.

describe('normalizeCourierRating', () => {
  it('maps camelCase raw fields → snake_case CourierRating', () => {
    const raw = {
      averageStars: 4.5,
      totalRatings: 2,
      ratings: [
        { deliveryId: 'dlv-1', stars: 4, comment: 'Быстро', createdAt: '2026-05-16T08:00:00Z' },
      ],
    };
    const out = normalizeCourierRating(raw);
    expect(out.average_stars).toBe(4.5);
    expect(out.total_ratings).toBe(2);
    expect(out.ratings[0].delivery_id).toBe('dlv-1');
    expect(out.ratings[0].stars).toBe(4);
    expect(out.ratings[0].comment).toBe('Быстро');
    expect(out.ratings[0].created_at).toBe('2026-05-16T08:00:00Z');
  });

  it('handles empty ratings array safely', () => {
    const out = normalizeCourierRating({ averageStars: 0, totalRatings: 0, ratings: [] });
    expect(out.average_stars).toBe(0);
    expect(out.total_ratings).toBe(0);
    expect(out.ratings).toEqual([]);
  });

  it('handles missing ratings field with fallback to []', () => {
    // @ts-expect-error — testing runtime robustness when ratings is undefined
    const out = normalizeCourierRating({ averageStars: 0, totalRatings: 0, ratings: undefined });
    expect(out.ratings).toEqual([]);
  });
});

describe('normalizeAvailableOrder', () => {
  it('maps all 7 camelCase fields → snake_case AvailableOrder', () => {
    const raw = {
      orderId: 'ord-av-1',
      deliveryId: 'dlv-av-1',
      deliveryAddress: 'Москва, Тверская, 1',
      totalPrice: 520,
      itemsCount: 2,
      zoneName: 'Test Zone',
      createdAt: '2026-05-16T07:00:00Z',
    };
    const out = normalizeAvailableOrder(raw);
    expect(out.order_id).toBe('ord-av-1');
    expect(out.delivery_id).toBe('dlv-av-1');
    expect(out.delivery_address).toBe('Москва, Тверская, 1');
    expect(out.total_price).toBe(520);
    expect(out.items_count).toBe(2);
    expect(out.zone_name).toBe('Test Zone');
    expect(out.created_at).toBe('2026-05-16T07:00:00Z');
  });

  it('maps items array (CRDR-07)', () => {
    const out = normalizeAvailableOrder({
      orderId: 'o1',
      deliveryId: 'd1',
      deliveryAddress: 'A',
      totalPrice: 100,
      itemsCount: 1,
      zoneName: 'Z',
      createdAt: 't',
      items: [{ name: 'Пицца', quantity: 2, price: 390 }],
    });
    expect(out.items).toEqual([{ name: 'Пицца', quantity: 2, price: 390 }]);
  });

  it('defaults items to [] when field absent (graceful — D-03)', () => {
    const out = normalizeAvailableOrder({
      orderId: 'o1',
      deliveryId: 'd1',
      deliveryAddress: 'A',
      totalPrice: 100,
      itemsCount: 0,
      zoneName: 'Z',
      createdAt: 't',
    });
    expect(out.items).toEqual([]);
  });
});

describe('normalizeCourierProfile', () => {
  it('maps camelCase wire → snake_case domain (RATE-05)', () => {
    const out = normalizeCourierProfile({
      firstName: 'Иван',
      lastName: 'Иванов',
      averageStars: 4.5,
      totalRatings: 12,
    });
    expect(out).toEqual({
      first_name: 'Иван',
      last_name: 'Иванов',
      average_stars: 4.5,
      total_ratings: 12,
    });
  });
});

describe('getCourierProfile', () => {
  it('resolves to normalized CourierProfile from MSW handler', async () => {
    const profile = await getCourierProfile('crr-1');
    expect(profile.first_name).toBe('Иван');
    expect(profile.last_name).toBe('Иванов');
    expect(profile.average_stars).toBe(4.5);
    expect(profile.total_ratings).toBe(12);
  });

  it('propagates a 404 (graceful fallback handled by caller — D-09)', async () => {
    server.use(
      http.get('/v1/couriers/:courierId/profile', () =>
        HttpResponse.json({ error: 'not found' }, { status: 404 }),
      ),
    );
    await expect(getCourierProfile('missing')).rejects.toBeDefined();
  });
});

describe('normalizeCourierStatus', () => {
  it('maps updatedAt → updated_at', () => {
    const raw = { id: 'crr-1', status: 'available', updatedAt: '2026-05-16T08:05:53.898844Z' };
    const out = normalizeCourierStatus(raw);
    expect(out.id).toBe('crr-1');
    expect(out.status).toBe('available');
    expect(out.updated_at).toBe('2026-05-16T08:05:53.898844Z');
  });
});

describe('getCourierRating', () => {
  it('returns normalized CourierRating from MSW handler', async () => {
    const rating = await getCourierRating('crr-1');
    expect(rating.average_stars).toBe(4.5);
    expect(rating.total_ratings).toBe(2);
    expect(rating.ratings[0].delivery_id).toBe('dlv-1');
  });
});

describe('updateCourierStatus', () => {
  it('happy path returns normalized CourierStatus with given status', async () => {
    const result = await updateCourierStatus('offline');
    expect(result.status).toBe('offline');
  });

  it('propagates HTTP 409 as a rejected promise (active_delivery_exists)', async () => {
    server.use(
      http.patch('*/v1/couriers/me/status', () =>
        HttpResponse.json(
          { code: 10, message: 'active_delivery_exists', details: [] },
          { status: 409 },
        ),
      ),
    );
    await expect(updateCourierStatus('offline')).rejects.toThrow();
  });
});

describe('listAvailableOrders', () => {
  it('returns normalized AvailableOrder array from MSW handler', async () => {
    const orders = await listAvailableOrders();
    expect(orders).toHaveLength(1);
    expect(orders[0].delivery_id).toBe('dlv-av-1');
    expect(orders[0].total_price).toBe(520);
  });
});

describe('getCourierDeliveries', () => {
  it('returns normalized Delivery array via reused normalizeDelivery (prefix stripped)', async () => {
    const deliveries = await getCourierDeliveries('crr-1');
    expect(deliveries).toHaveLength(1);
    // normalizeDelivery strips DELIVERY_STATUS_ prefix — proves reuse
    expect(deliveries[0].status).toBe('assigned');
  });
});

describe('updateOrderStatus', () => {
  it('returns normalized Order with status prefix stripped', async () => {
    const order = await updateOrderStatus('ord-1', 'ORDER_STATUS_PICKED_UP');
    // The MSW handler echoes the status back in the order; normalizeOrder strips the prefix
    expect(order.status).toBe('picked_up');
  });
});

describe('postCourierLocation', () => {
  it('resolves without throwing when MSW returns {success:true}', async () => {
    await expect(
      postCourierLocation({ courierId: 'c', orderId: 'o', lat: 55.7, lng: 37.6 }),
    ).resolves.toBeUndefined();
  });
});
