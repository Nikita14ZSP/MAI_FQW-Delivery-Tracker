import type { OrderItem } from '@/types/order';

export interface OrderItemsListProps {
  items: OrderItem[];
}

export function OrderItemsList({ items }: OrderItemsListProps) {
  const total = items.reduce((s, it) => s + it.price * it.quantity, 0);
  return (
    <div>
      <div className="flex flex-col divide-y divide-gray-100">
        {items.map((it, i) => (
          <div key={i} className="flex justify-between py-2 text-sm">
            <span className="text-gray-700">
              {it.name} × {it.quantity}
            </span>
            <span className="font-semibold text-gray-900">
              {it.price * it.quantity} ₽
            </span>
          </div>
        ))}
      </div>
      <div className="flex justify-between pt-3 text-base font-semibold">
        <span>Итого</span>
        <span>{total} ₽</span>
      </div>
    </div>
  );
}
