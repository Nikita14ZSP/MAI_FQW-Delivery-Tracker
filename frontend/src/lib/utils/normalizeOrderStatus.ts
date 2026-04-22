import type { Order, OrderRaw } from '@/types/order';
import type { OrderStatus } from '@/lib/constants';

export function normalizeOrderStatusString(
  raw: string | undefined | null,
): OrderStatus {
  if (!raw) return 'created';
  const stripped = raw.replace(/^ORDER_STATUS_/, '').toLowerCase();
  return stripped as OrderStatus;
}

export function normalizeOrder(order: OrderRaw): Order {
  // Spread is safe — Order is OrderRaw + status narrowing + deliveryId mapping.
  // delivery_id (snake_case from grpc-gateway JSON) → deliveryId (camelCase for app consumption).
  const { delivery_id, ...rest } = order;
  return {
    ...rest,
    status: normalizeOrderStatusString(order.status as string),
    deliveryId: delivery_id, // undefined-safe; preserved as optional
  };
}
