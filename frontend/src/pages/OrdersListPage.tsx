import { useEffect, useMemo } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';

import { useAuth } from '@/hooks/useAuth';
import { useToast } from '@/components/ui/use-toast';
import { listOrders } from '@/lib/api/orders';
import {
  ACTIVE_STATUSES,
  COMPLETED_STATUSES,
  type OrderStatus,
} from '@/lib/constants';

import { Button } from '@/components/ui/button';
import { EmptyOrders } from '@/components/orders/EmptyOrders';
import { OrderCard } from '@/components/orders/OrderCard';
import { RefreshButton } from '@/components/orders/RefreshButton';
import { SkeletonOrderCard } from '@/components/orders/SkeletonOrderCard';
import {
  StatusTabs,
  type StatusFilter,
} from '@/components/orders/StatusTabs';

const VALID_FILTERS: readonly StatusFilter[] = [
  'active',
  'completed',
  'all',
] as const;

function isValidFilter(raw: string | null): raw is StatusFilter {
  return raw !== null && (VALID_FILTERS as readonly string[]).includes(raw);
}

export function OrdersListPage() {
  const { user, logout } = useAuth();

  function handleLogout() {
    logout();
    navigate('/login', { replace: true });
  }
  const { toast } = useToast();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [params, setParams] = useSearchParams();

  const rawStatus = params.get('status');
  const filter: StatusFilter = isValidFilter(rawStatus) ? rawStatus : 'active';

  const setFilter = (next: StatusFilter) => {
    setParams({ status: next }, { replace: true });
  };

  const userId = user?.id;

  const {
    data: orders = [],
    isLoading,
    isError,
    isFetching,
    refetch,
  } = useQuery({
    queryKey: ['orders', userId],
    queryFn: () => listOrders(userId as string),
    enabled: !!userId,
  });

  useEffect(() => {
    if (isError) {
      toast({
        variant: 'destructive',
        title: 'Ошибка сервера. Попробуйте позже',
      });
    }
  }, [isError, toast]);

  const filtered = useMemo(() => {
    if (filter === 'all') return orders;
    const set: readonly OrderStatus[] =
      filter === 'active' ? ACTIVE_STATUSES : COMPLETED_STATUSES;
    return orders.filter((order) =>
      (set as readonly string[]).includes(order.status),
    );
  }, [orders, filter]);

  const handleRefresh = () => {
    queryClient.invalidateQueries({ queryKey: ['orders', userId] });
  };

  const handleRetry = () => {
    refetch();
  };

  return (
    <div className="min-h-screen bg-gray-50">
      <div className="mx-auto max-w-2xl px-4 py-6">
        <header className="mb-4 flex items-center justify-between gap-2">
          <h1 className="text-2xl font-semibold text-gray-900">Мои заказы</h1>
          <div className="flex items-center gap-2">
            <Button type="button" size="sm" onClick={() => navigate('/orders/new')}>
              Создать заказ
            </Button>
            <RefreshButton onClick={handleRefresh} isFetching={isFetching} />
            <Button type="button" size="sm" variant="outline" onClick={handleLogout}>
              Выйти
            </Button>
          </div>
        </header>
        <StatusTabs value={filter} onChange={setFilter} />

        {isLoading ? (
          <div className="flex flex-col gap-3" aria-busy="true">
            <SkeletonOrderCard />
            <SkeletonOrderCard />
            <SkeletonOrderCard />
          </div>
        ) : isError ? (
          <div className="flex flex-col items-center gap-3 py-12 text-center">
            <p className="text-sm text-red-600">Не удалось загрузить заказы</p>
            <RefreshButton onClick={handleRetry} isFetching={isFetching} />
          </div>
        ) : filtered.length === 0 ? (
          <EmptyOrders
            variant={filter === 'completed' ? 'completed' : 'active'}
          />
        ) : (
          <div className="flex flex-col gap-3">
            {filtered.map((order) => (
              <OrderCard
                key={order.id}
                order={order}
                onClick={() => navigate(`/orders/${order.id}`)}
              />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
