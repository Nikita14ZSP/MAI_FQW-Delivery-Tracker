import type { OrderStatus } from '@/lib/constants';

export type OrderStatusRaw =
  | 'ORDER_STATUS_UNSPECIFIED'
  | 'ORDER_STATUS_CREATED'
  | 'ORDER_STATUS_CONFIRMED'
  | 'ORDER_STATUS_ASSIGNED'
  | 'ORDER_STATUS_PICKED_UP'
  | 'ORDER_STATUS_IN_TRANSIT'
  | 'ORDER_STATUS_DELIVERED'
  | 'ORDER_STATUS_CANCELLED'
  | 'ORDER_STATUS_FAILED'
  | 'ORDER_STATUS_RETURNED';

export interface OrderItem {
  name: string;
  quantity: number;
  price: number;
}

export interface Coordinates {
  latitude: number;
  longitude: number;
}

// Wire format from grpc-gateway (camelCase + raw enum status).
export interface OrderRaw {
  id: string;
  userId: string;
  status: OrderStatusRaw | OrderStatus;
  deliveryAddress: string;
  deliveryCoordinates: Coordinates;
  items: OrderItem[];
  contactPhone: string;
  paymentMethod: string;
  createdAt: string;
  updatedAt: string;
  delivery_id?: string; // optional — only present once order has been assigned a delivery (Phase 6 BKND-04)
}

// Normalized for app consumption.
export interface Order {
  id: string;
  userId: string;
  status: OrderStatus;
  deliveryAddress: string;
  deliveryCoordinates: Coordinates;
  items: OrderItem[];
  contactPhone: string;
  paymentMethod: string;
  createdAt: string;
  updatedAt: string;
  deliveryId?: string; // normalized camelCase form of delivery_id from OrderRaw
}

export interface MenuCategory {
  id: string;
  name: string;
  sortOrder: number;
}

export interface MenuItem {
  id: string;
  categoryId: string;
  name: string;
  price: number;
  imageUrl: string;
  available: boolean;
}
