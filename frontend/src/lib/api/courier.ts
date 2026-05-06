import { api } from '@/lib/api';
import { normalizeDelivery } from '@/lib/api/deliveries';
import { normalizeOrder } from '@/lib/utils/normalizeOrderStatus';
import type { Delivery, DeliveryRaw } from '@/types/tracking';
import type { Order, OrderRaw } from '@/types/order';

// ---------------------------------------------------------------------------
// Raw types — EXACT camelCase grpc-gateway wire shapes (live-confirmed curl
// probes 2026-05-16; see 10-RESEARCH.md §"Live-Confirmed API Response Shapes")
// ---------------------------------------------------------------------------

/**
 * GET /v1/couriers/{courier_id}/rating — rating entry wire shape.
 * Live shape: { deliveryId, stars, comment, createdAt }
 */
export interface CourierRatingEntryRaw {
  deliveryId: string;
  stars: number;
  comment: string;
  createdAt: string;
}

/**
 * GET /v1/couriers/{courier_id}/rating — top-level wire envelope.
 * Live shape: { averageStars, totalRatings, ratings: [] }
 */
export interface CourierRatingRaw {
  averageStars: number;
  totalRatings: number;
  ratings: CourierRatingEntryRaw[];
}

/**
 * PATCH /v1/couriers/me/status — courier object in the { courier: ... } envelope.
 * Live shape: { id, status, updatedAt }
 */
export interface CourierStatusRaw {
  id: string;
  status: string;
  updatedAt: string;
}

/**
 * GET /v1/couriers/me/available-orders — preview item wire shape.
 * Live shape: { orderId, deliveryId, deliveryAddress, totalPrice, itemsCount, zoneName, createdAt }
 */
export interface AvailableOrderItemRaw {
  name: string;
  quantity: number;
  price: number;
}

export interface AvailableOrderPreviewRaw {
  orderId: string;
  deliveryId: string;
  deliveryAddress: string;
  totalPrice: number;
  itemsCount: number;
  zoneName: string;
  createdAt: string;
  items?: AvailableOrderItemRaw[];
}

export interface CourierProfileRaw {
  firstName: string;
  lastName: string;
  averageStars: number;
  totalRatings: number;
}

// ---------------------------------------------------------------------------
// Domain types — snake_case for consistency with Order/Delivery
// ---------------------------------------------------------------------------

export interface CourierRatingEntry {
  delivery_id: string;
  stars: number;
  comment: string;
  created_at: string;
}

export interface CourierRating {
  average_stars: number;
  total_ratings: number;
  ratings: CourierRatingEntry[];
}

export interface CourierStatus {
  id: string;
  status: string;
  updated_at: string;
}

export interface AvailableOrderItem {
  name: string;
  quantity: number;
  price: number;
}

export interface AvailableOrder {
  order_id: string;
  delivery_id: string;
  delivery_address: string;
  total_price: number;
  items_count: number;
  zone_name: string;
  created_at: string;
  items: AvailableOrderItem[];
}

export interface CourierProfile {
  first_name: string;
  last_name: string;
  average_stars: number;
  total_ratings: number;
}

// ---------------------------------------------------------------------------
// Normalizers — map grpc-gateway camelCase wire shape → snake_case domain type
// ---------------------------------------------------------------------------

/**
 * Normalize a single rating entry.
 * Wire: { deliveryId, stars, comment, createdAt }
 * Domain: { delivery_id, stars, comment, created_at }
 */
function normalizeRatingEntry(raw: CourierRatingEntryRaw): CourierRatingEntry {
  return {
    delivery_id: raw.deliveryId,
    stars: raw.stars,
    comment: raw.comment,
    created_at: raw.createdAt,
  };
}

/**
 * Normalize GET /v1/couriers/{courier_id}/rating response.
 * Wire: { averageStars, totalRatings, ratings: CourierRatingEntryRaw[] }
 * Domain: { average_stars, total_ratings, ratings: CourierRatingEntry[] }
 * ratings ?? [] guards against absent field in empty-rating response.
 */
export function normalizeCourierRating(raw: CourierRatingRaw): CourierRating {
  return {
    average_stars: raw.averageStars,
    total_ratings: raw.totalRatings,
    ratings: (raw.ratings ?? []).map(normalizeRatingEntry),
  };
}

/**
 * Normalize PATCH /v1/couriers/me/status courier object.
 * Wire: { id, status, updatedAt }
 * Domain: { id, status, updated_at }
 */
export function normalizeCourierStatus(raw: CourierStatusRaw): CourierStatus {
  return {
    id: raw.id,
    status: raw.status,
    updated_at: raw.updatedAt,
  };
}

/**
 * Normalize GET /v1/couriers/me/available-orders order preview item.
 * Wire: { orderId, deliveryId, deliveryAddress, totalPrice, itemsCount, zoneName, createdAt }
 * Domain: { order_id, delivery_id, delivery_address, total_price, items_count, zone_name, created_at }
 */
export function normalizeAvailableOrder(raw: AvailableOrderPreviewRaw): AvailableOrder {
  return {
    order_id: raw.orderId,
    delivery_id: raw.deliveryId,
    delivery_address: raw.deliveryAddress,
    total_price: raw.totalPrice,
    items_count: raw.itemsCount,
    zone_name: raw.zoneName,
    created_at: raw.createdAt,
    items: (raw.items ?? []).map((i) => ({
      name: i.name,
      quantity: i.quantity,
      price: i.price,
    })),
  };
}

export function normalizeCourierProfile(raw: CourierProfileRaw): CourierProfile {
  return {
    first_name: raw.firstName,
    last_name: raw.lastName,
    average_stars: raw.averageStars,
    total_ratings: raw.totalRatings,
  };
}

// ---------------------------------------------------------------------------
// API functions — all use `api` (axios instance, baseURL /v1, refresh interceptor)
// ---------------------------------------------------------------------------

/**
 * GET /v1/couriers/{courier_id}/rating
 * Returns normalized CourierRating (average_stars, total_ratings, ratings[]).
 */
export async function getCourierRating(courierId: string): Promise<CourierRating> {
  const { data } = await api.get<CourierRatingRaw>(`/couriers/${courierId}/rating`);
  return normalizeCourierRating(data);
}

/**
 * GET /v1/couriers/{courier_id}/profile (public, no auth — RATE-05)
 * Returns normalized CourierProfile (first_name, last_name, average_stars, total_ratings).
 */
export async function getCourierProfile(courierId: string): Promise<CourierProfile> {
  const { data } = await api.get<CourierProfileRaw>(
    `/couriers/${courierId}/profile`,
  );
  return normalizeCourierProfile(data);
}

/**
 * PATCH /v1/couriers/me/status
 * Switches courier between 'available' and 'offline'.
 * HTTP 409 { code:10, message:"active_delivery_exists" } propagates as a rejected
 * promise — do NOT swallow; caller must catch and display appropriate message.
 */
export async function updateCourierStatus(
  status: 'available' | 'offline',
): Promise<CourierStatus> {
  const { data } = await api.patch<{ courier: CourierStatusRaw }>('/couriers/me/status', {
    status,
  });
  return normalizeCourierStatus(data.courier);
}

/**
 * GET /v1/couriers/me/available-orders
 * Returns list of orders available for acceptance (PENDING, no courier assigned).
 * data.orders ?? [] guards against absent field.
 */
export async function listAvailableOrders(): Promise<AvailableOrder[]> {
  const { data } = await api.get<{ orders: AvailableOrderPreviewRaw[]; nextCursor: string }>(
    '/couriers/me/available-orders',
  );
  return (data.orders ?? []).map(normalizeAvailableOrder);
}

/**
 * POST /v1/couriers/me/deliveries/{delivery_id}/accept
 * Accepts an order. Returns normalized Delivery via imported normalizeDelivery.
 * HTTP 409 { code:10, message:"max_active_reached" } propagates as a rejected promise.
 */
export async function acceptOrder(deliveryId: string): Promise<Delivery> {
  const { data } = await api.post<{ delivery: DeliveryRaw }>(
    `/couriers/me/deliveries/${deliveryId}/accept`,
  );
  return normalizeDelivery(data.delivery);
}

/**
 * GET /v1/couriers/{courier_id}/deliveries
 * Returns active deliveries for the given courier.
 * Reuses normalizeDelivery from deliveries.ts — strips DELIVERY_STATUS_ prefix.
 */
export async function getCourierDeliveries(courierId: string): Promise<Delivery[]> {
  const { data } = await api.get<{ deliveries: DeliveryRaw[] }>(
    `/couriers/${courierId}/deliveries`,
  );
  return (data.deliveries ?? []).map(normalizeDelivery);
}

/**
 * PATCH /v1/orders/{id}/status
 * Updates order status. Caller passes the FULL enum with prefix e.g. 'ORDER_STATUS_PICKED_UP'.
 * Reuses normalizeOrder from normalizeOrderStatus.ts — strips ORDER_STATUS_ prefix.
 * Role courier is permitted for: assigned→picked_up→in_transit→delivered.
 */
export async function updateOrderStatus(orderId: string, status: string): Promise<Order> {
  const { data } = await api.patch<{ order: OrderRaw }>(`/orders/${orderId}/status`, { status });
  return normalizeOrder(data.order);
}

/**
 * POST /v1/tracking/location
 * Posts courier GPS coordinates. Body uses snake_case keys + nested coordinates
 * object — live-confirmed D-03 contract (see 10-CONTEXT.md D-03):
 * { courier_id, order_id, coordinates: { latitude, longitude } }
 * Backend rate-limit: ~1 req/5s — callers should use setInterval ≥ 6000ms.
 * Resolves on { success: true } response.
 */
export async function postCourierLocation(params: {
  courierId: string;
  orderId: string;
  lat: number;
  lng: number;
}): Promise<void> {
  await api.post('/tracking/location', {
    courier_id: params.courierId,
    order_id: params.orderId,
    coordinates: { latitude: params.lat, longitude: params.lng },
  });
}
