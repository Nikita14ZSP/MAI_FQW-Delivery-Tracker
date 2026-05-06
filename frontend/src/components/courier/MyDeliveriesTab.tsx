/**
 * MyDeliveriesTab — lists the courier's active deliveries (CRDR-04).
 *
 * Active = status in {assigned, picked_up, in_transit}.
 * Delivered/failed are excluded from the active list.
 *
 * Unlike AvailableOrdersTab, this tab is NOT offline-gated — the courier
 * can still see already-accepted deliveries while offline (D-06).
 * Only the GPS simulation control inside ActiveDeliveryCard is online-gated.
 *
 * Query key 'courier-deliveries' is shared with AvailableOrdersTab's
 * onSuccess invalidation, so accepting an order makes it appear here instantly.
 *
 * Advance mutation calls updateOrderStatus with the FULL ORDER_STATUS_* enum
 * (CRDR-05). On success invalidates ['courier-deliveries'] + shows toast.
 */

import {
  useQuery,
  useMutation,
  useQueryClient,
} from '@tanstack/react-query';
import { useAuth } from '@/hooks/useAuth';
import { useToast } from '@/components/ui/use-toast';
import { getCourierDeliveries, updateOrderStatus } from '@/lib/api/courier';
import { ActiveDeliveryCard } from './ActiveDeliveryCard';
import type { Delivery } from '@/types/tracking';

// ---------------------------------------------------------------------------
// Active status set (excludes terminal statuses delivered/failed)
// ---------------------------------------------------------------------------

const ACTIVE = new Set(['assigned', 'picked_up', 'in_transit']);

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

export interface MyDeliveriesTabProps {
  isOnline: boolean;
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function MyDeliveriesTab({ isOnline }: MyDeliveriesTabProps) {
  const { user } = useAuth();
  const courierId = user?.id ?? '';
  const qc = useQueryClient();
  const { toast } = useToast();

  // Fetch all deliveries for this courier (active + history)
  const { data: deliveries = [], isLoading } = useQuery({
    queryKey: ['courier-deliveries', courierId],
    queryFn: () => getCourierDeliveries(courierId),
    enabled: !!courierId,
    refetchInterval: 15_000,
  });

  // Filter to active deliveries only
  const active: Delivery[] = deliveries.filter((d) => ACTIVE.has(d.status));

  // Mutation: advance delivery status one step (CRDR-05)
  const advanceMutation = useMutation({
    mutationFn: ({ orderId, nextEnum }: { orderId: string; nextEnum: string }) =>
      updateOrderStatus(orderId, nextEnum),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['courier-deliveries'] });
      toast({ title: 'Статус обновлён' });
    },
    onError: () => {
      toast({ variant: 'destructive', title: 'Не удалось обновить статус' });
    },
  });

  return (
    <div className="space-y-4">
      {/* Header */}
      <h2 className="text-base font-semibold text-gray-900">Мои доставки</h2>

      {/* Body */}
      {isLoading ? (
        <div className="py-4 text-center text-sm text-gray-400">Загружаем…</div>
      ) : active.length === 0 ? (
        <div className="rounded-xl border border-dashed p-8 text-center text-sm text-gray-500">
          У вас нет активных доставок
        </div>
      ) : (
        <div className="space-y-3">
          {active.map((d) => (
            <ActiveDeliveryCard
              key={d.id}
              delivery={d}
              isOnline={isOnline}
              onAdvance={(orderId, nextEnum) => advanceMutation.mutate({ orderId, nextEnum })}
              isAdvancing={
                advanceMutation.isPending &&
                advanceMutation.variables?.orderId === d.order_id
              }
            />
          ))}
        </div>
      )}
    </div>
  );
}
