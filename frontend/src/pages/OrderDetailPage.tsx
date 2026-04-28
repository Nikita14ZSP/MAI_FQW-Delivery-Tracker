import { useEffect } from 'react';
import { Link, useParams } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { format, parseISO } from 'date-fns';
import { ru } from 'date-fns/locale';
import { MapPin } from 'lucide-react';

import { useAuth } from '@/hooks/useAuth';
import { useToast } from '@/components/ui/use-toast';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';

import { getOrder } from '@/lib/api/orders';
import { getDeliveryByOrderID } from '@/lib/api/deliveries';
import {
  CANCELLABLE_STATUSES,
  STEPPER_STATUSES,
  ACTIVE_STATUSES,
  FINAL_STATUSES,
} from '@/lib/constants';

import { StatusBadge } from '@/components/orders/StatusBadge';
import { StatusStepper } from '@/components/orders/StatusStepper';
import { StatusBanner } from '@/components/orders/StatusBanner';
import { OrderItemsList } from '@/components/orders/OrderItemsList';
import { CourierInfo } from '@/components/orders/CourierInfo';
import { RatingCard } from '@/components/orders/RatingCard';
import { CancelOrderDialog } from '@/components/orders/CancelOrderDialog';

const STEPPER_SET = new Set<string>(STEPPER_STATUSES);
const BANNER_SET = new Set<string>(['cancelled', 'failed', 'returned']);
const COURIER_SECTION_STATUSES = new Set<string>(
  // ACTIVE_STATUSES = [created, confirmed, assigned, picked_up, in_transit];
  // courier becomes relevant from 'assigned' onward (slice(2) = [assigned, picked_up, in_transit]).
  ACTIVE_STATUSES.slice(2),
);
// 'delivered' is NOT in ACTIVE_STATUSES — add it explicitly so the delivery
// query fires for delivered orders (needed for RATE-04 rating + courier name).
const DELIVERY_NEEDED_STATUSES = new Set<string>([
  ...ACTIVE_STATUSES.slice(2),
  'delivered',
]);

function formatCreatedAt(iso: string): string {
  try {
    return format(parseISO(iso), 'd MMMM yyyy, HH:mm', { locale: ru });
  } catch {
    return iso;
  }
}

export function OrderDetailPage() {
  const { id = '' } = useParams<{ id: string }>();
  const { user } = useAuth();
  const { toast } = useToast();

  const { data: order, isLoading, isError, error } = useQuery({
    queryKey: ['order', id],
    queryFn: () => getOrder(id),
    enabled: !!id,
  });

  // RATE-04 / D-05: fetch the delivery for assigned+ statuses (incl. delivered)
  // to wire the real courier name and the deliveryId for the rating POST.
  const deliveryQuery = useQuery({
    queryKey: ['delivery', 'by-order', id],
    queryFn: () => getDeliveryByOrderID(id),
    enabled: !!id && !!order && DELIVERY_NEEDED_STATUSES.has(order.status),
    retry: false,
  });
  const courierId = deliveryQuery.data?.courier_id ?? null;
  const courierName = courierId ? `Курьер #${courierId.slice(0, 8)}` : null;
  const ratingDeliveryId = order?.deliveryId ?? deliveryQuery.data?.id ?? null;

  const status404 =
    (error as { response?: { status?: number } } | null)?.response?.status ===
    404;

  useEffect(() => {
    if (isError && !status404) {
      toast({
        variant: 'destructive',
        title: 'Ошибка сервера. Попробуйте позже',
      });
    }
  }, [isError, status404, toast]);

  if (status404) {
    return (
      <div className="min-h-screen bg-gray-50">
        <div className="mx-auto max-w-2xl px-4 py-6">
          <Button asChild variant="ghost" size="sm" className="mb-4">
            <Link to="/orders">← Мои заказы</Link>
          </Button>
          <p className="mt-12 text-center text-base text-gray-700">
            Заказ не найден
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gray-50">
      <div className="mx-auto max-w-2xl px-4 py-6">
        <Button asChild variant="ghost" size="sm" className="mb-4">
          <Link to="/orders">← Мои заказы</Link>
        </Button>
        {isLoading || !order ? (
          <div className="flex flex-col gap-4">
            <Skeleton className="h-10 w-48" />
            <Skeleton className="h-32 w-full" />
            <Skeleton className="h-40 w-full" />
          </div>
        ) : (
          <>
            <header className="mb-6 flex items-start justify-between">
              <div className="flex flex-col gap-1">
                <h1 className="text-2xl font-semibold text-gray-900">
                  Заказ #{order.id.slice(0, 8)}
                </h1>
                <span className="text-sm text-gray-500">
                  {formatCreatedAt(order.createdAt)}
                </span>
              </div>
              <StatusBadge status={order.status} />
            </header>

            {STEPPER_SET.has(order.status) && (
              <Card className="mb-4 p-4">
                <StatusStepper currentStatus={order.status} />
              </Card>
            )}
            {BANNER_SET.has(order.status) && (
              <div className="mb-4">
                <StatusBanner
                  status={order.status as 'cancelled' | 'failed' | 'returned'}
                />
              </div>
            )}

            {/* RATE-04 D-01: inline rating card, prominent, delivered only. */}
            {order.status === 'delivered' && ratingDeliveryId && (
              <div className="mb-4">
                <RatingCard orderId={order.id} deliveryId={ratingDeliveryId} />
              </div>
            )}

            <Card className="mb-4 p-4">
              <p className="mb-2 text-sm font-semibold uppercase tracking-wide text-gray-500">
                Адрес доставки
              </p>
              <p className="text-base text-gray-900">
                <MapPin className="mr-2 inline h-4 w-4 text-gray-400" />
                {order.deliveryAddress}
              </p>
            </Card>

            <Card className="mb-4 p-4">
              <p className="mb-2 text-sm font-semibold uppercase tracking-wide text-gray-500">
                Состав заказа
              </p>
              <OrderItemsList items={order.items} />
            </Card>

            {/* Phase 9 D-02: CTA «Отследить на карте» — visible only in non-final statuses */}
            {!FINAL_STATUSES.includes(
              order.status as (typeof FINAL_STATUSES)[number],
            ) && (
              <Button
                asChild
                className="mb-4 w-full bg-orange-500 hover:bg-orange-600 text-white"
                size="lg"
              >
                <Link to={`/orders/${order.id}/track`}>
                  <MapPin className="mr-2 h-5 w-5" aria-hidden="true" />
                  Отследить на карте
                </Link>
              </Button>
            )}

            {(COURIER_SECTION_STATUSES.has(order.status) ||
              order.status === 'delivered') &&
              courierName && (
                <Card className="mb-4 p-4">
                  {/* RATE-04 D-05: real courier name from the delivery query. */}
                  <CourierInfo courierName={courierName} etaMinutes={null} />
                </Card>
              )}

            {(CANCELLABLE_STATUSES as readonly string[]).includes(
              order.status,
            ) &&
              user && (
                <CancelOrderDialog orderId={order.id} userId={user.id} />
              )}
          </>
        )}
      </div>
    </div>
  );
}
