import { api } from '@/lib/api';
import type { Delivery, DeliveryRaw } from '@/types/tracking';

/**
 * Normalize the grpc-gateway camelCase Delivery wire shape → snake_case `Delivery`
 * domain object at the API-client boundary. Mirrors the normalizeOrder pattern
 * (see lib/utils/normalizeOrderStatus.ts): strip the enum prefix + lowercase the
 * status. `assignedAt` is usually absent on initial assignment — leave it
 * undefined; never alias `createdAt` into `assigned_at` (quick-260516-72y).
 */
export function normalizeDelivery(raw: DeliveryRaw): Delivery {
  return {
    id: raw.id,
    order_id: raw.orderId,
    courier_id: raw.courierId,
    zone_id: raw.zoneId,
    status: (raw.status ?? '')
      .replace(/^DELIVERY_STATUS_/, '')
      .toLowerCase(),
    estimated_delivery: raw.estimatedDelivery,
    assigned_at: raw.assignedAt,
  };
}

/**
 * GET /v1/deliveries/by-order/{order_id}
 * Per Phase 6 BKND-04. Throws axios error on non-2xx (caller catches 404 for unassigned orders).
 * Backend returns camelCase (grpc-gateway default); normalized to the snake_case
 * `Delivery` domain type so callers (OrderTrackingPage) read .courier_id / .estimated_delivery.
 */
export async function getDeliveryByOrderID(orderId: string): Promise<Delivery> {
  const { data } = await api.get<{ delivery: DeliveryRaw }>(
    `/deliveries/by-order/${orderId}`,
  );
  return normalizeDelivery(data.delivery);
}
