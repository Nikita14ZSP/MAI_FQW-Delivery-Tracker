/**
 * useGpsSimulator.test.ts — TDD RED → GREEN for the in-browser GPS simulator.
 * Uses vi.useFakeTimers() for setInterval control; global MSW server for API mocking.
 * No per-file listen/close — uses global setup from src/test/setup.ts (Phase 9 convention).
 */
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createElement, type ReactNode } from 'react';
import { http, HttpResponse } from 'msw';
import { server } from '@/test/mocks/server';
import { useGpsSimulator } from './useGpsSimulator';

const BASE = '/v1';

// Helper: flush all pending microtasks (async state in React hooks)
async function flushMicrotasks() {
  await act(async () => {
    await Promise.resolve();
    await Promise.resolve();
  });
}

function createWrapper() {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return {
    wrapper: ({ children }: { children: ReactNode }) =>
      createElement(QueryClientProvider, { client: qc }, children),
    qc,
  };
}

const TARGET = { lat: 55.76, lng: 37.62 };
const DEPOT_LAT = 55.73;
const DEPOT_LNG = 37.58;

// Courier last location (normalized flat shape from MSW — already normalized by tracking.ts)
const LAST_LOCATION = {
  courierId: 'crr-1',
  orderId: 'ord-1',
  coordinates: { latitude: 55.7565, longitude: 37.619 },
  timestamp: '2026-05-15T12:10:00Z',
};

beforeEach(() => {
  vi.useFakeTimers();
});

afterEach(() => {
  vi.useRealTimers();
  vi.restoreAllMocks();
});

describe('useGpsSimulator', () => {
  it('1. enabled:false → start() is a no-op, postCourierLocation not called', async () => {
    let locationPostCalled = false;
    server.use(
      http.post(`${BASE}/tracking/location`, () => {
        locationPostCalled = true;
        return HttpResponse.json({ success: true });
      }),
    );

    const { wrapper } = createWrapper();
    const { result } = renderHook(
      () =>
        useGpsSimulator({
          courierId: 'crr-1',
          orderId: 'ord-1',
          targetCoords: TARGET,
          enabled: false,
        }),
      { wrapper },
    );

    await act(async () => {
      result.current.start();
    });

    await flushMicrotasks();

    // Advance 2 intervals
    await act(async () => {
      vi.advanceTimersByTime(12000);
    });

    expect(locationPostCalled).toBe(false);
    expect(result.current.isRunning).toBe(false);
  });

  it('2. enabled:true → start() resolves last location, advances one step, posts coordinates', async () => {
    let lastBody: unknown = null;

    server.use(
      http.get(`${BASE}/tracking/couriers/:courierId/location`, () =>
        HttpResponse.json({ location: LAST_LOCATION }),
      ),
      http.post(`${BASE}/tracking/location`, async ({ request }) => {
        lastBody = await request.json();
        return HttpResponse.json({ success: true });
      }),
    );

    const { wrapper } = createWrapper();
    const { result } = renderHook(
      () =>
        useGpsSimulator({
          courierId: 'crr-1',
          orderId: 'ord-1',
          targetCoords: TARGET,
          enabled: true,
        }),
      { wrapper },
    );

    // start() resolves last location async
    await act(async () => {
      result.current.start();
      await Promise.resolve();
      await Promise.resolve();
    });

    expect(result.current.isRunning).toBe(true);

    // Advance one interval tick
    await act(async () => {
      vi.advanceTimersByTime(6000);
      await Promise.resolve();
      await Promise.resolve();
    });

    expect(result.current.step).toBe(1);
    // Coordinates should be interpolated between last location and target
    expect(lastBody).toBeTruthy();
    const body = lastBody as { courier_id: string; coordinates: { latitude: number; longitude: number } };
    // After 1/30 steps, lat should be between last loc lat and target lat
    expect(body.courier_id).toBe('crr-1');
    // latitude should be between start (55.7565) and target (55.76)
    expect(body.coordinates.latitude).toBeGreaterThanOrEqual(55.7565);
    expect(body.coordinates.latitude).toBeLessThanOrEqual(55.76);
  });

  it('3. path completion: 30 steps → isRunning false, onDone called once', async () => {
    let callCount = 0;
    const onDone = vi.fn();

    server.use(
      http.get(`${BASE}/tracking/couriers/:courierId/location`, () =>
        HttpResponse.json({ location: LAST_LOCATION }),
      ),
      http.post(`${BASE}/tracking/location`, () => {
        callCount++;
        return HttpResponse.json({ success: true });
      }),
    );

    const { wrapper } = createWrapper();
    const { result } = renderHook(
      () =>
        useGpsSimulator({
          courierId: 'crr-1',
          orderId: 'ord-1',
          targetCoords: TARGET,
          enabled: true,
          onDone,
        }),
      { wrapper },
    );

    await act(async () => {
      result.current.start();
      await Promise.resolve();
      await Promise.resolve();
    });

    // Advance all 30 steps
    await act(async () => {
      vi.advanceTimersByTime(30 * 6000);
      await Promise.resolve();
      await Promise.resolve();
    });

    expect(callCount).toBe(30);
    expect(result.current.isRunning).toBe(false);
    expect(onDone).toHaveBeenCalledOnce();
  });

  it('4. stop() mid-run clears interval; no more posts after stop', async () => {
    let callCount = 0;

    server.use(
      http.get(`${BASE}/tracking/couriers/:courierId/location`, () =>
        HttpResponse.json({ location: LAST_LOCATION }),
      ),
      http.post(`${BASE}/tracking/location`, () => {
        callCount++;
        return HttpResponse.json({ success: true });
      }),
    );

    const { wrapper } = createWrapper();
    const { result } = renderHook(
      () =>
        useGpsSimulator({
          courierId: 'crr-1',
          orderId: 'ord-1',
          targetCoords: TARGET,
          enabled: true,
        }),
      { wrapper },
    );

    await act(async () => {
      result.current.start();
      await Promise.resolve();
      await Promise.resolve();
    });

    // Advance one tick
    await act(async () => {
      vi.advanceTimersByTime(6000);
      await Promise.resolve();
    });

    const countAfterOneTick = callCount;
    expect(countAfterOneTick).toBe(1);

    // Stop the simulator
    act(() => {
      result.current.stop();
    });

    expect(result.current.isRunning).toBe(false);

    // Advance two more ticks — should not post
    await act(async () => {
      vi.advanceTimersByTime(12000);
      await Promise.resolve();
    });

    expect(callCount).toBe(countAfterOneTick); // count didn't increase
  });

  it('5. null last-location → falls back to depot (55.7300, 37.5800) for interpolation', async () => {
    let firstBody: unknown = null;

    server.use(
      http.get(`${BASE}/tracking/couriers/:courierId/location`, () =>
        HttpResponse.json({ location: null }),
      ),
      http.post(`${BASE}/tracking/location`, async ({ request }) => {
        if (firstBody === null) {
          firstBody = await request.json();
        }
        return HttpResponse.json({ success: true });
      }),
    );

    const { wrapper } = createWrapper();
    const { result } = renderHook(
      () =>
        useGpsSimulator({
          courierId: 'crr-1',
          orderId: 'ord-1',
          targetCoords: TARGET,
          enabled: true,
        }),
      { wrapper },
    );

    await act(async () => {
      result.current.start();
      await Promise.resolve();
      await Promise.resolve();
    });

    // One step
    await act(async () => {
      vi.advanceTimersByTime(6000);
      await Promise.resolve();
      await Promise.resolve();
    });

    // frac = 1/30, lat = 55.7300 + (55.76 - 55.7300)/30
    const expectedLat = DEPOT_LAT + (TARGET.lat - DEPOT_LAT) / 30;
    const expectedLng = DEPOT_LNG + (TARGET.lng - DEPOT_LNG) / 30;

    expect(firstBody).toBeTruthy();
    const body = firstBody as { coordinates: { latitude: number; longitude: number } };
    expect(body.coordinates.latitude).toBeCloseTo(expectedLat, 4);
    expect(body.coordinates.longitude).toBeCloseTo(expectedLng, 4);
  });
});
