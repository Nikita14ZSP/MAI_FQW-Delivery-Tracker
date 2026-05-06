/**
 * useGpsSimulator — in-browser GPS movement simulator for the courier dashboard.
 *
 * Ports the linear-interpolation algorithm from tests/courier-sim.sh into a React hook:
 *   frac = step / STEPS
 *   lat  = start.lat + (target.lat - start.lat) * frac
 *   lng  = start.lng + (target.lng - start.lng) * frac
 *
 * Replaces the shell script for the VKR demo — the committee sees live movement
 * directly in the courier browser. Honestly labelled "Демо-симуляция GPS" (D-01).
 *
 * Gating (D-02): caller passes enabled = isOnline && isActiveDelivery.
 * The hook stops automatically when enabled drops (offline / delivered).
 * The interval id lives in a ref — NOT state — so ticks don't trigger re-renders (Pitfall 6).
 *
 * Backend rate-limit: ~1 req/5s, hence INTERVAL_MS = 6000.
 */

import { useCallback, useEffect, useRef, useState } from 'react';
import { getCourierLastLocation } from '@/lib/api/tracking';
import { postCourierLocation } from '@/lib/api/courier';

// ---------------------------------------------------------------------------
// Constants — module scope so tests can import and assert against them.
// ---------------------------------------------------------------------------

/** Total interpolation steps (mirrors courier-sim.sh default of 20 but extended to 30). */
export const STEPS = 30;

/** Milliseconds between GPS posts. Backend rate-limit is ~1 req/5 s. */
export const INTERVAL_MS = 6000;

/** Fallback start position used when getCourierLastLocation returns null.
 *  ~3 km south-west of the Moscow city centre (mirrors courier-sim.sh default offset). */
export const DEPOT_FALLBACK = { lat: 55.7300, lng: 37.5800 } as const;

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export interface UseGpsSimulatorOpts {
  courierId: string;
  orderId: string;
  /** Delivery coordinates — GPS target. When null, start() is a no-op (no target). */
  targetCoords: { lat: number; lng: number } | null;
  /** D-02 gate: should be isOnline && (status==='picked_up'||status==='in_transit'). */
  enabled: boolean;
  /** Called once when the simulator completes all STEPS. */
  onDone?: () => void;
}

export interface UseGpsSimulatorResult {
  /** Starts the simulation (no-op when disabled, already running, or no target). */
  start: () => void;
  /** Stops the simulation immediately; calling when not running is a no-op. */
  stop: () => void;
  /** True while the interval is active. */
  isRunning: boolean;
  /** Current step index (0..STEPS). Updated on every tick. */
  step: number;
}

// ---------------------------------------------------------------------------
// Hook
// ---------------------------------------------------------------------------

export function useGpsSimulator(opts: UseGpsSimulatorOpts): UseGpsSimulatorResult {
  // ---- Refs (do NOT cause re-renders) ----

  /** setInterval return value — stored in ref so ticks don't re-render (Pitfall 6). */
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  /** Current step counter (mirrors `step` state but accessible inside interval closure). */
  const stepRef = useRef(0);

  /** Resolved start position (last courier location or depot fallback). */
  const startPosRef = useRef<{ lat: number; lng: number } | null>(null);

  /** Latest opts snapshot kept in a ref so the interval closure reads fresh values
   *  (courierId/orderId/targetCoords/onDone) without re-creating the interval. */
  const optsRef = useRef(opts);

  // ---- State (drives re-renders for UI) ----
  const [isRunning, setIsRunning] = useState(false);
  const [step, setStep] = useState(0);

  // Keep optsRef in sync with every render cycle
  useEffect(() => {
    optsRef.current = opts;
  });

  // ---- stop (useCallback: stable identity, used in effects) ----
  const stop = useCallback((): void => {
    if (intervalRef.current !== null) {
      clearInterval(intervalRef.current);
      intervalRef.current = null;
    }
    setIsRunning(false);
  }, []);

  // ---- start ----
  const start = useCallback(async (): Promise<void> => {
    // D-02 gate: no-op when disabled, already running, or target not set
    if (!optsRef.current.enabled || intervalRef.current !== null || !optsRef.current.targetCoords) {
      return;
    }

    // Resolve start position from last known location; fall back to depot
    const last = await getCourierLastLocation(optsRef.current.courierId).catch(() => null);
    startPosRef.current = last
      ? { lat: last.lat, lng: last.lng }
      : { ...DEPOT_FALLBACK };

    stepRef.current = 0;
    setStep(0);
    setIsRunning(true);

    intervalRef.current = setInterval(async () => {
      stepRef.current += 1;
      const i = stepRef.current;
      setStep(i);

      const { lat: sLat, lng: sLng } = startPosRef.current!;
      const target = optsRef.current.targetCoords!;
      const frac = i / STEPS;
      const lat = sLat + (target.lat - sLat) * frac;
      const lng = sLng + (target.lng - sLng) * frac;

      try {
        await postCourierLocation({
          courierId: optsRef.current.courierId,
          orderId: optsRef.current.orderId,
          lat,
          lng,
        });
      } catch (e) {
        // A 429 rate-limit or transient error: log and continue, do NOT abort (D-02).
        console.warn('[GPS sim] postCourierLocation failed:', e);
      }

      // Completion: stop and invoke onDone callback
      if (i >= STEPS) {
        // Read stop callback via ref to avoid capturing stale closure
        if (intervalRef.current !== null) {
          clearInterval(intervalRef.current);
          intervalRef.current = null;
        }
        setIsRunning(false);
        optsRef.current.onDone?.();
      }
    }, INTERVAL_MS);
  }, [stop]); // stop is stable; we read all other opts from optsRef

  // ---- Cleanup on unmount ----
  useEffect(() => {
    return () => {
      if (intervalRef.current) {
        clearInterval(intervalRef.current);
      }
    };
  }, []);

  // ---- D-02: auto-stop when gating drops (offline / delivered) ----
  useEffect(() => {
    if (!opts.enabled && intervalRef.current !== null) {
      stop();
    }
  }, [opts.enabled, stop]);

  return { start, stop, isRunning, step };
}
