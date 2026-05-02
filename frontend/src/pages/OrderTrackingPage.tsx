import { useEffect, useMemo } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { ArrowLeft, CheckCircle2 } from 'lucide-react';
import { differenceInMinutes, parseISO } from 'date-fns';

import { getOrder } from '@/lib/api/orders';
import { getDeliveryByOrderID } from '@/lib/api/deliveries';
import {
  getOrderLocationHistory,
  getCourierLastLocation,
} from '@/lib/api/tracking';
import { getCourierProfile } from '@/lib/api/courier';
import { useOrderTracking } from '@/hooks/useOrderTracking';
import { TrackingMap } from '@/components/tracking/TrackingMap';
import { ConnectionStatusChip } from '@/components/tracking/ConnectionStatusChip';
import { EtaChip } from '@/components/tracking/EtaChip';
import { WaitingForCourier } from '@/components/tracking/WaitingForCourier';
import { TrackingBottomSheet } from '@/components/tracking/TrackingBottomSheet';
import { StatusBadge } from '@/components/orders/StatusBadge';
import { CourierInfo } from '@/components/orders/CourierInfo';
import { FINAL_STATUSES, ASSIGNED_STATUSES } from '@/lib/constants';
import { useToast } from '@/components/ui/use-toast';
import type { LocationPoint } from '@/types/tracking';

export function OrderTrackingPage() {
  const { id: orderId = '' } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { toast } = useToast();

  // Seed: order (drives status + deliveryId)
  const orderQuery = useQuery({
    queryKey: ['order', orderId],
    queryFn: () => getOrder(orderId),
    enabled: !!orderId,
  });

  const order = orderQuery.data ?? null;
  const status = order?.status;
  // CRITICAL: camelCase deliveryId (normalizeOrder maps delivery_id → deliveryId per Plan 09-02)
  const deliveryId = order?.deliveryId ?? null;

  // Seed: delivery (for courier_id + estimated_delivery ETA) — only when assigned+
  const deliveryQuery = useQuery({
    queryKey: ['delivery', 'by-order', orderId],
    queryFn: () => getDeliveryByOrderID(orderId),
    enabled:
      !!orderId &&
      !!status &&
      (ASSIGNED_STATUSES as readonly string[]).includes(status),
    retry: false,
  });

  const courierId = deliveryQuery.data?.courier_id ?? null;
  const estimatedDelivery = deliveryQuery.data?.estimated_delivery ?? null;

  // Seed: location history (Polyline) — requires courier_id
  const historyQuery = useQuery({
    queryKey: ['tracking', 'history', courierId, orderId],
    queryFn: () => getOrderLocationHistory(courierId!, orderId, 50),
    enabled: !!courierId && !!orderId,
    retry: false,
  });

  // Seed: latest cached courier location (initial marker)
  const courierLocationQuery = useQuery({
    queryKey: ['tracking', 'courier-location', courierId],
    queryFn: () => getCourierLastLocation(courierId!),
    enabled: !!courierId,
    retry: false,
  });

  // CTRK-04: public courier profile (ФИО + ★). Graceful: retry:false → on
  // 404/error fall back to «Курьер» without breaking the tracking page (D-09).
  const profileQuery = useQuery({
    queryKey: ['courier', 'profile', courierId],
    queryFn: () => getCourierProfile(courierId!),
    enabled: !!courierId,
    retry: false,
    staleTime: 60_000,
  });

  // WS subscription (always; backend will hold until tracking events occur)
  const tracking = useOrderTracking(orderId);

  // Merge initial REST history with live WS history
  const mergedHistory = useMemo<LocationPoint[]>(() => {
    const seed = historyQuery.data ?? [];
    return [...seed, ...tracking.history];
  }, [historyQuery.data, tracking.history]);

  // Choose marker source: prefer WS last, fallback to REST cached
  const currentLocation: LocationPoint | null =
    tracking.lastLocation ?? courierLocationQuery.data ?? null;

  // ETA minutes — for CourierInfo prop. Computed each render (Date.now is impure for useMemo;
  // EtaChip already owns the 60s tick — this value is only "snapshot at render" for CourierInfo text).
  const etaMinutes: number | null = estimatedDelivery
    ? Math.max(0, differenceInMinutes(parseISO(estimatedDelivery), new Date()))
    : null;

  // CRITICAL ADAPTER: order.deliveryCoordinates has {latitude, longitude}
  // TrackingMap expects destination as {lat, lng} (Leaflet short-form)
  const destination = useMemo(() => {
    if (!order?.deliveryCoordinates) return null;
    return {
      lat: order.deliveryCoordinates.latitude,
      lng: order.deliveryCoordinates.longitude,
    };
  }, [order]);

  // Edge state: cancelled / failed / returned — redirect immediately (D-30)
  useEffect(() => {
    if (!status) return;
    if (status === 'cancelled') {
      toast({ title: 'Заказ отменён' });
      navigate(`/orders/${orderId}`, { replace: true });
    } else if (status === 'failed') {
      toast({ title: 'Сбой доставки', variant: 'destructive' });
      navigate(`/orders/${orderId}`, { replace: true });
    } else if (status === 'returned') {
      toast({ title: 'Заказ возвращён' });
      navigate(`/orders/${orderId}`, { replace: true });
    }
  }, [status, orderId, navigate, toast]);

  // Edge state: delivered — show overlay 3s then redirect (D-29).
  // Derived from status (no useState in effect — avoids cascading renders lint).
  const showDeliveredOverlay = status === 'delivered';
  useEffect(() => {
    if (status !== 'delivered') return;
    const t = setTimeout(() => {
      navigate(`/orders/${orderId}`, { replace: true });
    }, 3000);
    return () => clearTimeout(t);
  }, [status, orderId, navigate]);

  // Loading
  if (orderQuery.isLoading || !order) {
    return (
      <div className="flex h-screen items-center justify-center">
        <p className="text-gray-500">Загрузка...</p>
      </div>
    );
  }

  // If lander on a final non-delivered status — useEffect redirects; render nothing meanwhile
  if (
    FINAL_STATUSES.includes(status as (typeof FINAL_STATUSES)[number]) &&
    status !== 'delivered'
  ) {
    return null;
  }

  const isWaitingForCourier = !courierId; // delivery not assigned yet (created/confirmed)
  // CTRK-04: real ФИО from public profile; «Курьер» fallback on 404/error (D-09).
  const courierDisplayName = courierId
    ? profileQuery.data
      ? `${profileQuery.data.first_name} ${profileQuery.data.last_name}`
      : 'Курьер'
    : null;
  const courierStars = profileQuery.data?.average_stars ?? null;

  return (
    <div className="flex flex-col h-screen overflow-hidden">
      {/* Sticky header (D-23 zone 1) */}
      <header
        className="flex items-center gap-3 px-4 py-3 bg-white border-b border-gray-200 shadow-sm"
        style={{ zIndex: 1100 }}
      >
        <Link
          to={`/orders/${orderId}`}
          aria-label="Назад к заказу"
          className="flex items-center gap-1 text-sm text-gray-600 hover:text-gray-900"
        >
          <ArrowLeft className="h-5 w-5" />
        </Link>
        {status && <StatusBadge status={status} />}
        <div className="ml-auto flex items-center gap-2">
          <ConnectionStatusChip
            state={
              tracking.connectionState === 'connected'
                ? 'connected'
                : tracking.connectionState === 'offline'
                  ? 'offline'
                  : 'reconnecting'
            }
          />
          <EtaChip etaIso={estimatedDelivery} />
        </div>
      </header>

      {/* Map (D-23 zone 2) */}
      <main className="relative flex-1 min-h-0">
        {destination && (
          <TrackingMap
            destination={destination}
            lastLocation={currentLocation}
            history={mergedHistory}
            className="h-full w-full"
          />
        )}
      </main>

      {/* Bottom sheet (D-23 zone 3) */}
      <TrackingBottomSheet defaultExpanded>
        {isWaitingForCourier ? (
          <WaitingForCourier />
        ) : (
          <div className="space-y-2">
            <CourierInfo
              courierName={courierDisplayName}
              etaMinutes={etaMinutes}
              averageStars={courierStars}
            />
            {order.deliveryAddress && (
              <p className="text-sm text-gray-600">{order.deliveryAddress}</p>
            )}
          </div>
        )}
      </TrackingBottomSheet>

      {/* Delivered overlay (D-29) */}
      {showDeliveredOverlay && (
        <div
          role="dialog"
          aria-label="Заказ доставлен"
          className="fixed inset-0 bg-black/40 flex items-center justify-center"
          style={{ zIndex: 2000 }}
        >
          <div className="bg-white rounded-2xl px-8 py-6 shadow-2xl flex flex-col items-center gap-3">
            <CheckCircle2
              className="h-12 w-12 text-green-500"
              aria-hidden="true"
            />
            <p className="text-lg font-medium text-gray-900">Заказ доставлен!</p>
          </div>
        </div>
      )}

      {/* deliveryId exposed for debug attribute (helps Playwright / smoke) */}
      {deliveryId && (
        <span data-testid="delivery-id" data-delivery-id={deliveryId} hidden />
      )}
    </div>
  );
}
