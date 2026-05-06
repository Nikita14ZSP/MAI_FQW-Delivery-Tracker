import { useState } from 'react';
import { MapPin } from 'lucide-react';
import { Card } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { TimeAgo } from '@/components/orders/TimeAgo';
import type { AvailableOrder } from '@/lib/api/courier';

export interface AvailableOrderCardProps {
  order: AvailableOrder;
  onAccept: (deliveryId: string) => void;
  isAccepting: boolean;
}

export function AvailableOrderCard({
  order,
  onAccept,
  isAccepting,
}: AvailableOrderCardProps) {
  const [expanded, setExpanded] = useState(false);
  const hasItems = order.items.length > 0;

  return (
    <Card className="rounded-xl p-4 transition-shadow duration-150 hover:shadow-md">
      <div className="flex items-start justify-between gap-2">
        <div className="flex items-center text-sm text-gray-700 min-w-0">
          <MapPin className="mr-1 inline h-3 w-3 shrink-0 text-gray-400" />
          <span className="truncate">{order.delivery_address}</span>
        </div>
        <TimeAgo iso={order.created_at} />
      </div>
      <div className="mt-2 flex items-center justify-between gap-2">
        <div className="flex items-center gap-1 text-xs text-gray-500">
          <span>{order.items_count} поз.</span>
          <span>&bull;</span>
          <span>{order.zone_name}</span>
        </div>
        <span className="text-sm font-semibold text-gray-900">
          {order.total_price} ₽
        </span>
      </div>

      {expanded && hasItems && (
        <ul className="mt-3 space-y-1 text-sm text-gray-700">
          {order.items.map((it, idx) => (
            <li key={idx} className="flex justify-between gap-2">
              <span className="truncate">
                {it.name} × {it.quantity}
              </span>
              <span className="shrink-0">{it.price} ₽</span>
            </li>
          ))}
        </ul>
      )}

      <div className="mt-3 flex gap-2">
        {hasItems && (
          <Button
            type="button"
            variant="outline"
            onClick={() => setExpanded((v) => !v)}
            className="flex-1"
          >
            {expanded ? 'Скрыть' : 'Подробнее'}
          </Button>
        )}
        <Button
          onClick={() => onAccept(order.delivery_id)}
          disabled={isAccepting}
          className="flex-1 bg-orange-500 hover:bg-orange-600 text-white"
        >
          {isAccepting ? 'Принимаю…' : 'Принять'}
        </Button>
      </div>
    </Card>
  );
}
