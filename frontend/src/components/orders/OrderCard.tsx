import type { KeyboardEvent } from 'react';
import { MapPin } from 'lucide-react';
import { Card } from '@/components/ui/card';
import { StatusBadge } from './StatusBadge';
import { TimeAgo } from './TimeAgo';
import type { Order } from '@/types/order';

export interface OrderCardProps {
  order: Order;
  onClick: () => void;
}

export function OrderCard({ order, onClick }: OrderCardProps) {
  const total = order.items.reduce(
    (sum, item) => sum + item.price * item.quantity,
    0,
  );
  const itemsPreview = order.items
    .slice(0, 2)
    .map((item) => item.name)
    .join(', ');

  const handleKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault();
      onClick();
    }
  };

  return (
    <Card
      role="button"
      tabIndex={0}
      onClick={onClick}
      onKeyDown={handleKeyDown}
      className="cursor-pointer rounded-xl p-4 transition-shadow duration-150 hover:shadow-md focus:outline-none focus-visible:ring-2 focus-visible:ring-[var(--ring)]"
    >
      <div className="flex items-start justify-between gap-2">
        <StatusBadge status={order.status} />
        <TimeAgo iso={order.createdAt} />
      </div>
      <div className="mt-2 flex items-center text-sm text-gray-700">
        <MapPin className="mr-1 inline h-3 w-3 shrink-0 text-gray-400" />
        <span className="truncate">{order.deliveryAddress}</span>
      </div>
      <div className="mt-2 flex items-center justify-between gap-2">
        <span className="truncate text-xs text-gray-500">{itemsPreview}</span>
        <span className="text-sm font-semibold text-gray-900">{total} ₽</span>
      </div>
    </Card>
  );
}
