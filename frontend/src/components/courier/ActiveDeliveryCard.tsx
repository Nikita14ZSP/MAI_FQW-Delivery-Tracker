/**
 * ActiveDeliveryCard — shows a single active delivery for the courier.
 *
 * Shows:
 * - Delivery address (resolved from the linked order via useQuery)
 * - StatusBadge for the current delivery status
 * - A single «next stage» button whose label and enum change by status (CRDR-05):
 *     assigned   → «Забрал заказ»   → ORDER_STATUS_PICKED_UP
 *     picked_up  → «Начать доставку» → ORDER_STATUS_IN_TRANSIT
 *     in_transit → «Доставлено»      → ORDER_STATUS_DELIVERED
 *   Terminal statuses (delivered, failed) render no button.
 * - An in-browser GPS movement simulator «Начать движение» shown only when
 *   isOnline AND status is picked_up | in_transit AND order coords are loaded.
 *   Honestly labelled «Демо-симуляция GPS» (D-01, replaces tests/courier-sim.sh).
 *
 * GPS gating (D-02): enabled = isOnline && status in {picked_up, in_transit}.
 * The simulator auto-stops when enabled drops (handled by useGpsSimulator's effect).
 */

import { useQuery } from '@tanstack/react-query';
import { Loader2, MapPin, Navigation } from 'lucide-react';
import { Card } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { StatusBadge } from '@/components/orders/StatusBadge';
import { useAuth } from '@/hooks/useAuth';
import { useGpsSimulator } from '@/hooks/useGpsSimulator';
import { getOrder } from '@/lib/api/orders';
import type { Delivery } from '@/types/tracking';

// ---------------------------------------------------------------------------
// Next-stage state machine (CRDR-05)
// ---------------------------------------------------------------------------

const NEXT_STAGE: Record<string, { label: string; enum: string }> = {
  assigned:   { label: 'Забрал заказ',    enum: 'ORDER_STATUS_PICKED_UP' },
  picked_up:  { label: 'Начать доставку', enum: 'ORDER_STATUS_IN_TRANSIT' },
  in_transit: { label: 'Доставлено',      enum: 'ORDER_STATUS_DELIVERED' },
};

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

export interface ActiveDeliveryCardProps {
  delivery: Delivery;
  /** Passed from parent (CourierPage → MyDeliveriesTab). Controls GPS block visibility. */
  isOnline: boolean;
  /** Called when courier taps the next-stage button. */
  onAdvance: (orderId: string, nextEnum: string) => void;
  /** True while the advance mutation is in-flight (shows spinner + disabled). */
  isAdvancing: boolean;
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function ActiveDeliveryCard({
  delivery,
  isOnline,
  onAdvance,
  isAdvancing,
}: ActiveDeliveryCardProps) {
  const { user } = useAuth();
  const courierId = user?.id ?? '';

  // Resolve linked order to get address + GPS target coordinates
  const { data: order } = useQuery({
    queryKey: ['order', delivery.order_id],
    queryFn: () => getOrder(delivery.order_id),
    enabled: !!delivery.order_id,
  });

  // Next-stage button config for this delivery status
  const next = NEXT_STAGE[delivery.status];

  // GPS target from order.deliveryCoordinates (camelCase, live-confirmed)
  const target = order?.deliveryCoordinates
    ? { lat: order.deliveryCoordinates.latitude, lng: order.deliveryCoordinates.longitude }
    : null;

  // D-02 gating: GPS only runs when online AND delivery is mid-flight
  const gpsEnabled = isOnline && (delivery.status === 'picked_up' || delivery.status === 'in_transit');

  const gps = useGpsSimulator({
    courierId,
    orderId: delivery.order_id,
    targetCoords: target,
    enabled: gpsEnabled,
  });

  return (
    <Card className="rounded-xl p-4">
      {/* Top row: address + status badge */}
      <div className="flex items-start justify-between gap-2">
        <div className="flex min-w-0 items-center gap-1 text-sm text-gray-700">
          <MapPin className="h-3 w-3 shrink-0 text-gray-400" />
          <span className="truncate">{order?.deliveryAddress ?? '—'}</span>
        </div>
        <StatusBadge status={delivery.status as Parameters<typeof StatusBadge>[0]['status']} />
      </div>

      {/* Next-stage button (hidden for terminal statuses: delivered, failed) */}
      {next && (
        <Button
          onClick={() => onAdvance(delivery.order_id, next.enum)}
          disabled={isAdvancing}
          className="mt-3 w-full"
        >
          {isAdvancing ? <Loader2 className="h-4 w-4 animate-spin" /> : next.label}
        </Button>
      )}

      {/* GPS simulation block — only when online + active mid-delivery + target loaded */}
      {gpsEnabled && target && (
        <div className="mt-3">
          <p className="text-xs text-gray-400">Демо-симуляция GPS</p>
          {!gps.isRunning ? (
            <Button
              variant="outline"
              onClick={gps.start}
              disabled={gps.isRunning}
              className="mt-1 w-full"
            >
              <Navigation className="h-4 w-4" />
              Начать движение
            </Button>
          ) : (
            <div className="mt-1 flex items-center gap-2">
              <span className="flex-1 text-sm text-gray-600">
                Движение… {gps.step}/30
              </span>
              <Button variant="ghost" size="sm" onClick={gps.stop}>
                Стоп
              </Button>
            </div>
          )}
        </div>
      )}
    </Card>
  );
}
