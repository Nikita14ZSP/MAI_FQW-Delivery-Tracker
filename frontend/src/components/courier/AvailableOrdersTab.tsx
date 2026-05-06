import { RefreshCw } from 'lucide-react';
import {
  useQuery,
  useMutation,
  useQueryClient,
} from '@tanstack/react-query';
import { Button } from '@/components/ui/button';
import { useToast } from '@/components/ui/use-toast';
import { listAvailableOrders, acceptOrder } from '@/lib/api/courier';
import type { AvailableOrder } from '@/lib/api/courier';
import { AvailableOrderCard } from './AvailableOrderCard';

export interface AvailableOrdersTabProps {
  isOnline: boolean;
}

interface AcceptContext {
  snapshot: AvailableOrder[] | undefined;
}

export function AvailableOrdersTab({ isOnline }: AvailableOrdersTabProps) {
  const qc = useQueryClient();
  const { toast } = useToast();

  const {
    data: orders = [],
    refetch,
    isLoading,
  } = useQuery({
    queryKey: ['available-orders'],
    queryFn: listAvailableOrders,
    refetchInterval: 15_000,
    enabled: isOnline,
  });

  const acceptMutation = useMutation<
    Awaited<ReturnType<typeof acceptOrder>>,
    unknown,
    string,
    AcceptContext
  >({
    mutationFn: (deliveryId: string) => acceptOrder(deliveryId),
    onMutate: async (deliveryId) => {
      await qc.cancelQueries({ queryKey: ['available-orders'] });
      const snapshot = qc.getQueryData<AvailableOrder[]>(['available-orders']);
      qc.setQueryData<AvailableOrder[]>(['available-orders'], (old) =>
        (old ?? []).filter((o) => o.delivery_id !== deliveryId),
      );
      return { snapshot };
    },
    onError: (_e, _v, ctx) => {
      if (ctx?.snapshot) {
        qc.setQueryData(['available-orders'], ctx.snapshot);
      }
      toast({ variant: 'destructive', title: 'Не удалось принять заказ' });
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['courier-deliveries'] });
      toast({ title: 'Заказ принят' });
    },
  });

  // Offline gate — shown after hooks so hook call order stays stable across isOnline transitions
  if (!isOnline) {
    return (
      <div className="rounded-xl border border-dashed p-8 text-center text-sm text-gray-500">
        Переключитесь в режим «Работаю», чтобы принимать заказы
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h2 className="text-base font-semibold text-gray-900">
          Доступные заказы
        </h2>
        <Button
          variant="ghost"
          size="sm"
          onClick={() => refetch()}
          className="gap-1"
        >
          <RefreshCw className="h-4 w-4" />
          Обновить
        </Button>
      </div>

      {/* Body */}
      {isLoading ? (
        <div className="py-4 text-center text-sm text-gray-400">
          Загружаем…
        </div>
      ) : orders.length === 0 ? (
        <div className="rounded-xl border border-dashed p-8 text-center">
          <p className="text-sm text-gray-500 mb-3">Нет доступных заказов</p>
          <Button variant="outline" size="sm" onClick={() => refetch()}>
            Обновить
          </Button>
        </div>
      ) : (
        <div className="space-y-3">
          {orders.map((o) => (
            <AvailableOrderCard
              key={o.delivery_id}
              order={o}
              onAccept={(id) => acceptMutation.mutate(id)}
              isAccepting={
                acceptMutation.isPending &&
                acceptMutation.variables === o.delivery_id
              }
            />
          ))}
        </div>
      )}
    </div>
  );
}
