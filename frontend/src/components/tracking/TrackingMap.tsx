import { useEffect, useRef } from 'react';
import { MapContainer, TileLayer, Marker, Polyline, useMap } from 'react-leaflet';
import L from 'leaflet';
import { Crosshair } from 'lucide-react';
import 'leaflet/dist/leaflet.css';
import '@/lib/leaflet-icon-fix';
import type { LocationPoint } from '@/types/tracking';

const MOSCOW_CENTER: L.LatLngTuple = [55.7558, 37.6173];

/**
 * Guard against malformed backend coordinates reaching Leaflet (quick-260516-e30).
 * A LocationPoint whose lat/lng are undefined / NaN / Infinity must NOT be fed
 * into Polyline/Marker/setLatLng — Leaflet throws `TypeError: undefined is not
 * an object (evaluating 'e[0]')`, the React tree unmounts → blank white page.
 * Treat any non-finite point as absent (graceful degradation).
 */
function isFinitePoint<T extends { lat: number; lng: number }>(
  p: T | null | undefined,
): p is T {
  return !!p && Number.isFinite(p.lat) && Number.isFinite(p.lng);
}

function createCourierIcon(): L.DivIcon {
  // Lucide Bike SVG (inline; per RESEARCH §CourierIcon)
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" fill="none" stroke="white" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><circle cx="18.5" cy="17.5" r="3.5"/><circle cx="5.5" cy="17.5" r="3.5"/><circle cx="15" cy="5" r="1"/><path d="M12 17.5V14l-3-3 4-3 2 3h2"/></svg>`;
  return L.divIcon({
    className: '',
    html: `<div style="width:40px;height:40px;background:#F97316;border:3px solid white;border-radius:50%;display:flex;align-items:center;justify-content:center;box-shadow:0 4px 12px rgba(0,0,0,0.25);">${svg}</div>`,
    iconSize: [40, 40],
    iconAnchor: [20, 20],
  });
}

function createDestinationPin(): L.DivIcon {
  // Blue map-pin (matches MapPicker.tsx createPinIcon style)
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 24 24" fill="#2563EB" stroke="white" stroke-width="1.5"><path d="M12 2C8 2 5 5 5 9c0 5 7 13 7 13s7-8 7-13c0-4-3-7-7-7z"/><circle cx="12" cy="9" r="2.5" fill="white"/></svg>`;
  return L.divIcon({
    className: '',
    html: svg,
    iconSize: [32, 32],
    iconAnchor: [16, 32],
  });
}

function InitialFitBounds({
  destination,
  courier,
}: {
  destination: { lat: number; lng: number };
  courier: LocationPoint | null;
}) {
  const map = useMap();
  const didFitRef = useRef(false);
  useEffect(() => {
    if (didFitRef.current || !isFinitePoint(courier) || !isFinitePoint(destination))
      return;
    didFitRef.current = true;
    map.fitBounds(
      [
        [destination.lat, destination.lng],
        [courier.lat, courier.lng],
      ],
      { padding: [50, 50] },
    );
  }, [courier, destination, map]);
  return null;
}

function RecenterButton({
  destination,
  courier,
}: {
  destination: { lat: number; lng: number };
  courier: LocationPoint | null;
}) {
  const map = useMap();
  const handleClick = () => {
    if (!isFinitePoint(courier)) {
      map.setView([destination.lat, destination.lng], 14);
      return;
    }
    map.fitBounds(
      [
        [destination.lat, destination.lng],
        [courier.lat, courier.lng],
      ],
      { padding: [50, 50] },
    );
  };
  return (
    <button
      type="button"
      onClick={handleClick}
      aria-label="Центрировать карту"
      className="absolute bottom-4 right-4 z-[500] h-10 w-10 rounded-full bg-white shadow-lg border border-gray-200 flex items-center justify-center hover:bg-gray-50"
    >
      <Crosshair className="h-5 w-5 text-gray-700" aria-hidden="true" />
    </button>
  );
}

export interface TrackingMapProps {
  destination: { lat: number; lng: number };
  lastLocation: LocationPoint | null;
  history: LocationPoint[];
  className?: string;
}

export function TrackingMap({
  destination,
  lastLocation,
  history,
  className,
}: TrackingMapProps) {
  const markerRef = useRef<L.Marker | null>(null);
  const tweenRef = useRef<number | null>(null);
  const fromPosRef = useRef<[number, number] | null>(null);

  // Treat a non-finite lastLocation/history point as absent so malformed
  // backend data degrades gracefully instead of throwing into Leaflet
  // (quick-260516-e30).
  const courier = isFinitePoint(lastLocation) ? lastLocation : null;
  const finiteHistory = history.filter(isFinitePoint);

  // PLSH-01 (D-07..D-09): rAF linear tween over the imperative markerRef.
  // First fix = instant placement; subsequent fixes glide ~5s; a new fix
  // mid-tween cancels and restarts from the last tween's start (fromPosRef);
  // cleanup cancels any in-flight tween (Pitfall 1: avoid setLatLng on a
  // destroyed Leaflet marker after unmount/navigation).
  useEffect(() => {
    if (!courier || !markerRef.current) return;

    // First fix: no previous position → place instantly, no animation (D-09).
    if (!fromPosRef.current) {
      markerRef.current.setLatLng([courier.lat, courier.lng]);
      fromPosRef.current = [courier.lat, courier.lng];
      return;
    }

    // New fix mid-tween: cancel in-flight, restart from last tween start (D-08).
    if (tweenRef.current !== null) {
      cancelAnimationFrame(tweenRef.current);
      tweenRef.current = null;
    }

    const from = fromPosRef.current;
    const to: [number, number] = [courier.lat, courier.lng];
    const duration = 5000; // ~5s, ≈ GPS interval (D-08)
    const startTime = performance.now();

    const tick = (now: number): void => {
      if (!markerRef.current) return;
      const t = Math.min((now - startTime) / duration, 1); // linear easing
      const lat = from[0] + (to[0] - from[0]) * t;
      const lng = from[1] + (to[1] - from[1]) * t;
      markerRef.current.setLatLng([lat, lng]);
      if (t < 1) {
        tweenRef.current = requestAnimationFrame(tick);
      } else {
        tweenRef.current = null;
        fromPosRef.current = to;
      }
    };

    tweenRef.current = requestAnimationFrame(tick);

    return () => {
      if (tweenRef.current !== null) {
        cancelAnimationFrame(tweenRef.current);
        tweenRef.current = null;
      }
    };
  }, [courier]);

  return (
    <div
      role="application"
      aria-label="Карта отслеживания заказа"
      className={className}
    >
      <MapContainer
        center={courier ? [courier.lat, courier.lng] : MOSCOW_CENTER}
        zoom={11}
        attributionControl={false}
        className="h-full w-full"
      >
        <TileLayer
          url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
          attribution=""
        />
        <Marker
          position={[destination.lat, destination.lng]}
          icon={createDestinationPin()}
        />
        {courier && (
          <Marker
            ref={(instance) => {
              markerRef.current = instance ?? null;
            }}
            position={[courier.lat, courier.lng]}
            icon={createCourierIcon()}
          />
        )}
        {finiteHistory.length > 1 && (
          <Polyline
            positions={finiteHistory.map((p) => [p.lat, p.lng] as L.LatLngTuple)}
            pathOptions={{ color: '#F97316', weight: 4, opacity: 0.7 }}
          />
        )}
        <InitialFitBounds destination={destination} courier={courier} />
        <RecenterButton destination={destination} courier={courier} />
      </MapContainer>
    </div>
  );
}
