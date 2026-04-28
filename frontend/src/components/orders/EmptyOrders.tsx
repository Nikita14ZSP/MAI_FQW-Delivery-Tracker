import { Link } from 'react-router-dom';
import { ClipboardList, Plus, ShoppingBag } from 'lucide-react';
import { Button } from '@/components/ui/button';

export interface EmptyOrdersProps {
  variant: 'active' | 'completed';
}

export function EmptyOrders({ variant }: EmptyOrdersProps) {
  if (variant === 'completed') {
    return (
      <div className="flex flex-col items-center justify-center px-4 py-12 text-center">
        <ClipboardList className="mb-4 h-12 w-12 text-gray-300" />
        <h2 className="text-lg font-semibold text-gray-900">
          Нет завершённых заказов
        </h2>
        <p className="mt-1 max-w-[240px] text-sm text-gray-500">
          Здесь появятся доставленные и отменённые заказы
        </p>
      </div>
    );
  }
  return (
    <div className="flex flex-col items-center justify-center px-4 py-12 text-center">
      <ShoppingBag className="mb-4 h-12 w-12 text-gray-300" />
      <h2 className="text-lg font-semibold text-gray-900">
        У вас пока нет заказов
      </h2>
      <p className="mt-1 max-w-[240px] text-sm text-gray-500">
        Создайте первый заказ — это займёт пару минут
      </p>
      <Button asChild className="mt-6">
        <Link to="/orders/new">
          <Plus className="mr-2 h-4 w-4" />
          Создать заказ
        </Link>
      </Button>
    </div>
  );
}
