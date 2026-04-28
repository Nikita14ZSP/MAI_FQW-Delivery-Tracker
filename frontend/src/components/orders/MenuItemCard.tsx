import { useState } from 'react';
import { UtensilsCrossed } from 'lucide-react';
import { cn } from '@/lib/utils';
import { QtyStepper } from './QtyStepper';
import type { MenuItem } from '@/types/order';

export interface MenuItemCardProps {
  item: MenuItem;
  quantity: number;
  onIncrement: (item: MenuItem) => void;
  onDecrement: (item: MenuItem) => void;
}

export function MenuItemCard({
  item,
  quantity,
  onIncrement,
  onDecrement,
}: MenuItemCardProps) {
  const [imgError, setImgError] = useState(false);
  const selected = quantity > 0;
  return (
    <div
      className={cn(
        'relative rounded-lg border bg-white overflow-hidden transition-shadow hover:shadow-sm',
        selected
          ? 'border-blue-600 ring-2 ring-blue-600/20'
          : 'border-gray-200',
      )}
    >
      <div className="relative aspect-[4/3] w-full bg-gray-100">
        {item.imageUrl && !imgError ? (
          <img
            src={item.imageUrl}
            alt={item.name}
            loading="lazy"
            onError={() => setImgError(true)}
            className="h-full w-full object-cover"
          />
        ) : (
          <div className="flex h-full w-full items-center justify-center">
            <UtensilsCrossed className="h-6 w-6 text-gray-300" />
          </div>
        )}
        {selected && (
          <span className="absolute top-2 right-2 flex h-5 w-5 items-center justify-center rounded-full bg-blue-600 text-xs font-semibold text-white">
            {quantity}
          </span>
        )}
      </div>
      <div className="p-3">
        <p className="truncate text-sm font-semibold text-gray-900">
          {item.name}
        </p>
        <p className="mt-1 text-sm font-semibold text-gray-900">
          {item.price} ₽
        </p>
        <div className="mt-2">
          <QtyStepper
            value={quantity}
            onIncrement={() => onIncrement(item)}
            onDecrement={() => onDecrement(item)}
          />
        </div>
      </div>
    </div>
  );
}
