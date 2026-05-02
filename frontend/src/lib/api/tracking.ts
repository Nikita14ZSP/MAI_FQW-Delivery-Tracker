import { api } from '@/lib/api';
import type { LocationPoint, LocationPointRaw } from '@/types/tracking';

/**
 * Normalize the grpc-gateway nested camelCase location wire shape →
 * flat snake_case `LocationPoint` domain object at the API-client boundary.
 * Mirrors the normalizeDelivery pattern (see lib/api/deliveries.ts) for the
 * Phase 9 tracking seed clients (quick-260516-e30):
 * - courierId            → courier_id
 * - orderId              → order_id   ('' when absent; LocationPoint.order_id is required)
 * - coordinates.latitude → lat
 * - coordinates.longitude→ lng
 * - timestamp            → recorded_at
 * - (no wire id)         → id ('' — LocationPoint.id is required; matches existing
 *                           fixtures which use id: '')
 */
export function normalizeLocationPoint(raw: LocationPointRaw): LocationPoint {
  return {
    id: '',
    courier_id: raw.courierId,
    order_id: raw.orderId ?? '',
    lat: raw.coordinates.latitude,
    lng: raw.coordinates.longitude,
    recorded_at: raw.timestamp,
  };
}

/**
 * GET /v1/tracking/history/{courier_id}?order_id={oid}&limit={N}
 * Per Pitfall 7 — endpoint is keyed by courier_id (NOT order_id); order_id and limit are query params.
 * Backend returns nested camelCase (grpc-gateway default); each item is
 * normalized to the flat snake_case `LocationPoint` domain type, then sorted
 * ASC by recorded_at (client-side sort for safety).
 */
export async function getOrderLocationHistory(
  courierId: string,
  orderId: string,
  limit = 50,
): Promise<LocationPoint[]> {
  const { data } = await api.get<{ locations: LocationPointRaw[] }>(
    `/tracking/history/${courierId}`,
    { params: { order_id: orderId, limit } },
  );
  const points = (data.locations ?? []).map(normalizeLocationPoint);
  return points.sort((a, b) => (a.recorded_at < b.recorded_at ? -1 : 1));
}

/**
 * GET /v1/tracking/couriers/{courier_id}/location
 * Returns last cached position from Redis. null if no position cached.
 * Backend returns the nested camelCase envelope `{location: LocationPointRaw | null}`;
 * normalized to the flat snake_case `LocationPoint` domain type (quick-260516-e30).
 */
export async function getCourierLastLocation(
  courierId: string,
): Promise<LocationPoint | null> {
  const { data } = await api.get<{ location: LocationPointRaw | null }>(
    `/tracking/couriers/${courierId}/location`,
  );
  return data.location ? normalizeLocationPoint(data.location) : null;
}
