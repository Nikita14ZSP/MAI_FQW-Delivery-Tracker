/** GPS точка от backend (1-to-1 c services/tracking-service/internal/domain/location.go LocationPoint).
 *  ВАЖНО: tracking domain в backend использует short-form lat/lng (НЕ latitude/longitude как Order.deliveryCoordinates).
 *  Это намеренно: Leaflet нативно использует [lat, lng] tuple. */
export interface LocationPoint {
  id: string;
  courier_id: string;
  order_id: string;
  lat: number;
  lng: number;
  recorded_at: string; // RFC3339
}

/** Wire format from grpc-gateway for a location point (nested camelCase).
 *  This is the ACTUAL backend JSON shape (live-confirmed via courier JWT probe —
 *  quick-260516-e30): GET /v1/tracking/history/{courier_id} returns
 *  `{locations:[LocationPointRaw]}` and GET /v1/tracking/couriers/{id}/location
 *  returns `{location: LocationPointRaw | null}`. normalizeLocationPoint() maps
 *  it INTO the flat snake_case `LocationPoint` domain type at the API-client
 *  boundary (same pattern as DeliveryRaw/normalizeDelivery, quick-260516-72y).
 *  NOTE: the wire shape carries NO id; LocationPoint.id defaults to ''. */
export interface LocationPointRaw {
  courierId: string;
  orderId?: string;
  coordinates: { latitude: number; longitude: number };
  timestamp: string; // RFC3339
}

/** WS connection lifecycle state (per 09-CONTEXT.md D-12) */
export type ConnectionState =
  | 'connecting'
  | 'connected'
  | 'reconnecting'
  | 'offline'
  | 'closed';

/** Return type of useOrderTracking hook (per 09-RESEARCH §Pattern 1) */
export interface UseOrderTrackingResult {
  connectionState: ConnectionState;
  lastLocation: LocationPoint | null;
  history: LocationPoint[];
}

/** Wire format from grpc-gateway for Delivery (camelCase + raw enum status).
 *  This is the ACTUAL backend JSON shape; normalizeDelivery() maps it to the
 *  snake_case `Delivery` domain type at the API-client boundary (see quick-260516-72y).
 *  NOTE: backend sends `createdAt` (not `assignedAt`) on initial assignment —
 *  do NOT alias createdAt into assigned_at. */
export interface DeliveryRaw {
  id: string;
  orderId: string;
  courierId: string;
  zoneId?: string;
  status: string;
  estimatedDelivery?: string; // RFC3339
  createdAt?: string; // RFC3339
  assignedAt?: string; // RFC3339 — usually absent on initial assignment
}

/** Delivery domain object — 1-to-1 c proto/delivery/v1/delivery.proto Delivery message.
 *  Used for ETA seed (estimated_delivery) and courier_id resolution. */
export interface Delivery {
  id: string;
  order_id: string;
  courier_id: string;
  zone_id?: string;
  status: string;
  estimated_delivery?: string; // RFC3339
  assigned_at?: string;
}
