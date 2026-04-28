import { useQuery } from '@tanstack/react-query';
import { listItems } from '@/lib/api/menu';
import { Skeleton } from '@/components/ui/skeleton';
import { MenuItemCard } from './MenuItemCard';
import type { MenuItem } from '@/types/order';

export interface MenuGridProps {
  activeCategoryId: string;
  getQty: (item: MenuItem) => number;
  onIncrement: (item: MenuItem) => void;
  onDecrement: (item: MenuItem) => void;
}

export function MenuGrid({
  activeCategoryId,
  getQty,
  onIncrement,
  onDecrement,
}: MenuGridProps) {
  const { data: items = [], isLoading } = useQuery({
    queryKey: ['menu', 'items', activeCategoryId || 'all'],
    queryFn: () => listItems(activeCategoryId || undefined),
    staleTime: Infinity,
    gcTime: Infinity,
  });

  if (isLoading) {
    return (
      <div className="mt-3 grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-2 xl:grid-cols-3">
        {Array.from({ length: 6 }).map((_, i) => (
          <div
            key={i}
            className="rounded-lg border border-gray-200 bg-white overflow-hidden"
          >
            <Skeleton className="aspect-[4/3] w-full rounded-none" />
            <div className="p-3 space-y-2">
              <Skeleton className="h-4 w-3/4" />
              <Skeleton className="h-4 w-1/3" />
            </div>
          </div>
        ))}
      </div>
    );
  }

  return (
    <div className="mt-3 grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-2 xl:grid-cols-3">
      {items.map((item) => (
        <MenuItemCard
          key={item.id}
          item={item}
          quantity={getQty(item)}
          onIncrement={onIncrement}
          onDecrement={onDecrement}
        />
      ))}
    </div>
  );
}
