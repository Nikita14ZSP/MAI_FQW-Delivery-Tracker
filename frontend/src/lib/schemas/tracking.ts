import { z } from 'zod';

// LocationUpdateData (matches LocationUpdateData in Go)
export const LocationUpdateDataSchema = z.object({
  courier_id: z.string(),
  order_id: z.string(),
  lat: z.number(),
  lng: z.number(),
  timestamp: z.string(), // RFC3339
});

// DeliveryAssignedEvent (matches pkg/kafka/events.go DeliveryAssignedEvent)
export const DeliveryAssignedDataSchema = z.object({
  event_id: z.string().optional(),
  delivery_id: z.string(),
  order_id: z.string(),
  user_id: z.string().optional(),
  courier_id: z.string(),
  zone_id: z.string().optional(),
  eta: z.string(), // RFC3339
  assigned_at: z.string().optional(),
});

export const OrderUpdatedDataSchema = z.object({
  order_id: z.string(),
  user_id: z.string().optional(),
  old_status: z.string(),
  new_status: z.string(),
  updated_at: z.string().optional(),
});

export const DeliveryStatusDataSchema = z.object({
  delivery_id: z.string(),
  order_id: z.string(),
  courier_id: z.string(),
  old_status: z.string(),
  new_status: z.string(),
  updated_at: z.string().optional(),
});

export const OrderCreatedDataSchema = z.object({
  order_id: z.string(),
  user_id: z.string().optional(),
  address: z.string().optional(),
  lat: z.number().optional(),
  lng: z.number().optional(),
  created_at: z.string().optional(),
});

// Envelope per services/tracking-service/internal/domain/location.go WSMessage
export const WSMessageSchema = z.discriminatedUnion('type', [
  z.object({ type: z.literal('location_update'), data: LocationUpdateDataSchema }),
  z.object({ type: z.literal('order_created'), data: OrderCreatedDataSchema }),
  z.object({ type: z.literal('order_status_change'), data: OrderUpdatedDataSchema }),
  z.object({ type: z.literal('delivery_assigned'), data: DeliveryAssignedDataSchema }),
  z.object({ type: z.literal('delivery_status'), data: DeliveryStatusDataSchema }),
]);

export type WSMessage = z.infer<typeof WSMessageSchema>;

// Delivery (matches proto/delivery/v1/delivery.proto Delivery message)
export const DeliverySchema = z.object({
  id: z.string(),
  order_id: z.string(),
  courier_id: z.string(),
  zone_id: z.string().optional(),
  status: z.string(),
  estimated_delivery: z.string().optional(),
  assigned_at: z.string().optional(),
});

export type DeliverySchemaType = z.infer<typeof DeliverySchema>;
