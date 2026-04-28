import { api } from '@/lib/api';
import type { Order, OrderRaw } from '@/types/order';
import { normalizeOrder } from '@/lib/utils/normalizeOrderStatus';

interface ListOrdersResponse {
  orders: OrderRaw[];
  pagination?: { total: number; page: number; pageSize: number };
}

interface SingleOrderResponse {
  order: OrderRaw;
}

export interface CreateOrderPayload {
  user_id: string;
  delivery_address: string;
  delivery_coordinates: { latitude: number; longitude: number };
  items: Array<{ name: string; quantity: number; price: number }>;
  contact_phone: string;
  payment_method: 'cash' | 'card_on_delivery' | 'online';
}

export async function createOrder(payload: CreateOrderPayload): Promise<Order> {
  const { data } = await api.post<SingleOrderResponse>('/orders', payload);
  return normalizeOrder(data.order);
}

export async function listOrders(userId: string): Promise<Order[]> {
  const { data } = await api.get<ListOrdersResponse>('/orders', {
    params: { user_id: userId, page_size: 100 },
  });
  return (data.orders ?? []).map(normalizeOrder);
}

export async function getOrder(id: string): Promise<Order> {
  const { data } = await api.get<SingleOrderResponse>(`/orders/${id}`);
  return normalizeOrder(data.order);
}

export async function cancelOrder(
  id: string,
  reason = 'client_cancel',
): Promise<Order> {
  const { data } = await api.post<SingleOrderResponse>(
    `/orders/${id}/cancel`,
    { reason },
  );
  return normalizeOrder(data.order);
}
