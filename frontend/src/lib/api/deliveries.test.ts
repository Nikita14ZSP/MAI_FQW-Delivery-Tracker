import { describe, it, expect } from 'vitest';
import { server } from '@/test/mocks/server';
import { http, HttpResponse } from 'msw';
import { getDeliveryByOrderID, normalizeDelivery } from './deliveries';
import type { DeliveryRaw } from '@/types/tracking';

// MSW server lifecycle (listen/reset/close) is handled globally in src/test/setup.ts.
// Per-test override pattern: server.use(...) — reset is automatic in global afterEach.

describe('normalizeDelivery', () => {
  it('maps camelCase raw fields → snake_case Delivery', () => {
    const raw: DeliveryRaw = {
      id: 'dlv-1',
      orderId: 'ord-1',
      courierId: 'crr-1',
      zoneId: 'zone-1',
      status: 'DELIVERY_STATUS_ASSIGNED',
      estimatedDelivery: '2026-05-16T02:11:18.531098Z',
      createdAt: '2026-05-16T01:41:13.994964Z',
      assignedAt: '2026-05-16T01:45:00Z',
    };
    const out = normalizeDelivery(raw);
    expect(out.id).toBe('dlv-1');
    expect(out.order_id).toBe('ord-1');
    expect(out.courier_id).toBe('crr-1');
    expect(out.zone_id).toBe('zone-1');
    expect(out.estimated_delivery).toBe('2026-05-16T02:11:18.531098Z');
    expect(out.assigned_at).toBe('2026-05-16T01:45:00Z');
  });

  it('strips DELIVERY_STATUS_ prefix and lowercases status', () => {
    const out = normalizeDelivery({
      id: 'd',
      orderId: 'o',
      courierId: 'c',
      status: 'DELIVERY_STATUS_ASSIGNED',
    });
    expect(out.status).toBe('assigned');
  });

  it('is idempotent for already-normalized status', () => {
    const out = normalizeDelivery({
      id: 'd',
      orderId: 'o',
      courierId: 'c',
      status: 'assigned',
    });
    expect(out.status).toBe('assigned');
  });

  it('leaves optional fields undefined when absent (does NOT map createdAt → assigned_at)', () => {
    const out = normalizeDelivery({
      id: 'd',
      orderId: 'o',
      courierId: 'c',
      status: 'DELIVERY_STATUS_ASSIGNED',
      createdAt: '2026-05-16T01:41:13.994964Z',
    });
    expect(out.zone_id).toBeUndefined();
    expect(out.estimated_delivery).toBeUndefined();
    expect(out.assigned_at).toBeUndefined();
  });
});

describe('getDeliveryByOrderID', () => {
  it('returns a normalized snake_case Delivery for an existing order', async () => {
    const delivery = await getDeliveryByOrderID('ord-1');
    expect(delivery.id).toBe('dlv-mock-1');
    expect(delivery.order_id).toBe('ord-1');
    expect(delivery.courier_id).toBe('crr-mock-1');
    expect(delivery.zone_id).toBe('zone-1');
    expect(delivery.status).toBe('assigned');
    expect(delivery.estimated_delivery).toBe('2026-05-16T02:11:18.531098Z');
  });

  it('throws on 404 (delivery not yet created)', async () => {
    server.use(
      http.get('*/v1/deliveries/by-order/:orderId', () =>
        HttpResponse.json({ error: 'not_found' }, { status: 404 }),
      ),
    );
    await expect(getDeliveryByOrderID('ord-missing')).rejects.toThrow();
  });
});
