import { describe, it, expect } from 'vitest';
import {
  WSMessageSchema,
  DeliverySchema,
  LocationUpdateDataSchema,
} from './tracking';

describe('WSMessageSchema', () => {
  it('parses location_update envelope', () => {
    const result = WSMessageSchema.parse({
      type: 'location_update',
      data: {
        courier_id: 'crr-1',
        order_id: 'ord-1',
        lat: 55.7558,
        lng: 37.6173,
        timestamp: '2026-05-15T12:00:00Z',
      },
    });
    expect(result.type).toBe('location_update');
    if (result.type === 'location_update') {
      expect(result.data.lat).toBe(55.7558);
      expect(result.data.lng).toBe(37.6173);
    }
  });

  it('parses delivery_assigned with RFC3339 eta', () => {
    const result = WSMessageSchema.parse({
      type: 'delivery_assigned',
      data: {
        delivery_id: 'dlv-1',
        order_id: 'ord-1',
        courier_id: 'crr-1',
        eta: '2026-05-15T12:30:00Z',
      },
    });
    expect(result.type).toBe('delivery_assigned');
    if (result.type === 'delivery_assigned') {
      expect(result.data.eta).toBe('2026-05-15T12:30:00Z');
    }
  });

  it('parses order_status_change envelope', () => {
    const result = WSMessageSchema.parse({
      type: 'order_status_change',
      data: { order_id: 'ord-1', old_status: 'created', new_status: 'confirmed' },
    });
    expect(result.type).toBe('order_status_change');
  });

  it('parses delivery_status envelope', () => {
    const result = WSMessageSchema.parse({
      type: 'delivery_status',
      data: {
        delivery_id: 'dlv-1',
        order_id: 'ord-1',
        courier_id: 'crr-1',
        old_status: 'assigned',
        new_status: 'picked_up',
      },
    });
    expect(result.type).toBe('delivery_status');
  });

  it('parses order_created envelope', () => {
    const result = WSMessageSchema.parse({
      type: 'order_created',
      data: { order_id: 'ord-1' },
    });
    expect(result.type).toBe('order_created');
  });

  it('rejects unknown type', () => {
    expect(() =>
      WSMessageSchema.parse({ type: 'bogus', data: {} }),
    ).toThrow();
  });

  it('rejects location_update missing lat', () => {
    expect(() =>
      WSMessageSchema.parse({
        type: 'location_update',
        data: { courier_id: 'c', order_id: 'o', lng: 37, timestamp: 'x' },
      }),
    ).toThrow();
  });

  it('LocationUpdateDataSchema parses minimal valid payload', () => {
    const result = LocationUpdateDataSchema.parse({
      courier_id: 'c',
      order_id: 'o',
      lat: 1,
      lng: 2,
      timestamp: 't',
    });
    expect(result.lat).toBe(1);
  });
});

describe('DeliverySchema', () => {
  it('parses minimal delivery', () => {
    const result = DeliverySchema.parse({
      id: 'd',
      order_id: 'o',
      courier_id: 'c',
      status: 'assigned',
    });
    expect(result.id).toBe('d');
  });

  it('parses delivery with estimated_delivery RFC3339', () => {
    const result = DeliverySchema.parse({
      id: 'd',
      order_id: 'o',
      courier_id: 'c',
      status: 'assigned',
      estimated_delivery: '2026-05-15T12:30:00Z',
    });
    expect(result.estimated_delivery).toBe('2026-05-15T12:30:00Z');
  });
});
