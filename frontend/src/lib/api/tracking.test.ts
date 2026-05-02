import { describe, it, expect } from 'vitest';
import { server } from '@/test/mocks/server';
import { http, HttpResponse } from 'msw';
import {
  getOrderLocationHistory,
  getCourierLastLocation,
  normalizeLocationPoint,
} from './tracking';
import type { LocationPointRaw } from '@/types/tracking';

// MSW server lifecycle (listen/reset/close) is handled globally in src/test/setup.ts.
// Per-test override pattern: server.use(...) — reset is automatic in global afterEach.
// NOTE: getDeliveryByOrderID + normalizeDelivery tests live in ./deliveries.test.ts
// (boundary normalization contract — see quick-260516-72y).
// This file asserts the NORMALIZED flat snake_case LocationPoint contract:
// backend returns nested camelCase, normalizeLocationPoint maps it INTO the
// existing domain type (live-confirmed shape — see quick-260516-e30).

describe('normalizeLocationPoint', () => {
  it('maps nested camelCase raw fields → flat snake_case LocationPoint', () => {
    const raw: LocationPointRaw = {
      courierId: 'crr-1',
      orderId: 'ord-1',
      coordinates: { latitude: 55.765, longitude: 37.605 },
      timestamp: '2026-05-16T06:59:01.736830Z',
    };
    const out = normalizeLocationPoint(raw);
    expect(out.courier_id).toBe('crr-1');
    expect(out.order_id).toBe('ord-1');
    expect(out.lat).toBe(55.765);
    expect(out.lng).toBe(37.605);
    expect(out.recorded_at).toBe('2026-05-16T06:59:01.736830Z');
    // LocationPointRaw carries no id; LocationPoint.id is required → default ''
    expect(out.id).toBe('');
  });

  it("defaults order_id to '' when raw.orderId is absent", () => {
    const out = normalizeLocationPoint({
      courierId: 'crr-2',
      coordinates: { latitude: 55.7596, longitude: 37.597 },
      timestamp: '2026-05-16T06:58:55.692105Z',
    });
    expect(out.order_id).toBe('');
    expect(out.courier_id).toBe('crr-2');
    expect(out.lat).toBe(55.7596);
    expect(out.lng).toBe(37.597);
  });
});

describe('getOrderLocationHistory', () => {
  it('returns normalized LocationPoint[] mapped from nested camelCase', async () => {
    const points = await getOrderLocationHistory('crr-mock-1', 'order-mock-1', 50);
    expect(points.length).toBeGreaterThanOrEqual(2);
    expect(points[0]).toMatchObject({
      courier_id: expect.any(String),
      order_id: expect.any(String),
      lat: expect.any(Number),
      lng: expect.any(Number),
      recorded_at: expect.any(String),
    });
  });

  it('returns sorted by recorded_at ASC', async () => {
    const points = await getOrderLocationHistory('crr-mock-1', 'order-mock-1');
    for (let i = 1; i < points.length; i++) {
      expect(points[i].recorded_at >= points[i - 1].recorded_at).toBe(true);
    }
  });

  it('throws on 404 (no history yet)', async () => {
    server.use(
      http.get('*/v1/tracking/history/:courierId', () =>
        HttpResponse.json({ error: 'not_found' }, { status: 404 }),
      ),
    );
    await expect(getOrderLocationHistory('crr-mock-1', 'order-x')).rejects.toThrow();
  });
});

describe('getCourierLastLocation', () => {
  it('returns a normalized LocationPoint when envelope present', async () => {
    const loc = await getCourierLastLocation('crr-mock-1');
    expect(loc).not.toBeNull();
    expect(loc?.lat).toBeCloseTo(55.7565);
    expect(loc?.lng).toBeCloseTo(37.619);
    expect(loc?.courier_id).toBe('crr-mock-1');
    expect(loc?.recorded_at).toEqual(expect.any(String));
  });

  it('returns null when handler returns null location', async () => {
    server.use(
      http.get('*/v1/tracking/couriers/:courierId/location', () =>
        HttpResponse.json({ location: null }),
      ),
    );
    const loc = await getCourierLastLocation('crr-empty');
    expect(loc).toBeNull();
  });
});
