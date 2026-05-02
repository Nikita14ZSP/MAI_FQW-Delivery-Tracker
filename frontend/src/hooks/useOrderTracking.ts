import { useCallback, useEffect, useRef, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import {
  WSMessageSchema,
  type WSMessage,
} from '@/lib/schemas/tracking';
import type {
  ConnectionState,
  LocationPoint,
  UseOrderTrackingResult,
} from '@/types/tracking';
import { WS_BACKOFF_DELAYS_MS, WS_BACKOFF_JITTER } from '@/lib/constants';
import { ACCESS_TOKEN_KEY, api } from '@/lib/api';
import { toast } from '@/components/ui/use-toast';

/** Per D-12: 5+ failed reconnect attempts → connectionState='offline' (chip indicator). */
const OFFLINE_THRESHOLD = 5;

/** PLSH-03 (D-10/D-13): friendly Russian copy for key client-facing status transitions. */
const STATUS_TOAST_COPY: Record<string, string> = {
  assigned: 'Курьер назначен',
  picked_up: 'Курьер забрал ваш заказ',
  in_transit: 'Курьер в пути',
  delivered: 'Заказ доставлен',
  cancelled: 'Заказ отменён',
  failed: 'Доставка не состоялась',
};

/**
 * dispatchMessage — pure dispatcher over the 5 WS message types (D-35).
 * Side effects are confined to setters + QueryClient.invalidateQueries.
 */
function dispatchMessage(
  msg: WSMessage,
  ctx: {
    orderId: string;
    setLastLocation: (p: LocationPoint) => void;
    setHistory: (updater: (prev: LocationPoint[]) => LocationPoint[]) => void;
    qc: ReturnType<typeof useQueryClient>;
  },
): void {
  switch (msg.type) {
    case 'location_update': {
      const point: LocationPoint = {
        // synthetic id — backend WS payload doesn't include id
        id: '',
        courier_id: msg.data.courier_id,
        order_id: msg.data.order_id,
        lat: msg.data.lat,
        lng: msg.data.lng,
        recorded_at: msg.data.timestamp,
      };
      ctx.setLastLocation(point);
      ctx.setHistory((prev) => [...prev, point]);
      break;
    }
    case 'order_status_change': {
      // PLSH-03 (D-10..D-13): toast on key client-facing transitions.
      // Defensive normalization — backend may send raw enum 'ORDER_STATUS_DELIVERED'
      // or stripped 'delivered'; map both. Does NOT modify shared use-toast / TOAST_LIMIT.
      const newStatus = String(msg.data.new_status)
        .replace(/^ORDER_STATUS_/, '')
        .toLowerCase();
      const copy = STATUS_TOAST_COPY[newStatus];
      if (copy) {
        toast({
          title: copy,
          variant: newStatus === 'failed' ? 'destructive' : 'default',
        });
      }
      ctx.qc.invalidateQueries({ queryKey: ['order', ctx.orderId] });
      break;
    }
    case 'delivery_assigned':
      ctx.qc.invalidateQueries({ queryKey: ['order', ctx.orderId] });
      ctx.qc.invalidateQueries({ queryKey: ['delivery', msg.data.delivery_id] });
      break;
    case 'delivery_status':
      ctx.qc.invalidateQueries({ queryKey: ['order', ctx.orderId] });
      break;
    case 'order_created':
      // not a Phase 9 concern (tracking page exists only after order is created)
      break;
  }
}

/**
 * useOrderTracking — React hook managing the full WebSocket lifecycle for a
 * single order. Opens on mount, dispatches by message type, exponential
 * backoff reconnect with jitter, refresh-mutex on auth-class close codes
 * (1008 / 4401), graceful cleanup on unmount.
 *
 * URL contract: ws[s]://host/ws/orders/{orderId}?token=...
 * — matches nginx /ws/ proxy_pass + api-gateway /ws/orders/{order_id} route.
 *
 * See 09-CONTEXT.md D-04..D-11, D-35 and 09-RESEARCH.md §Pattern 1/§Pattern 5.
 */
export function useOrderTracking(orderId: string): UseOrderTrackingResult {
  const qc = useQueryClient();
  const [connectionState, setConnectionState] = useState<ConnectionState>('connecting');
  const [lastLocation, setLastLocation] = useState<LocationPoint | null>(null);
  const [history, setHistory] = useState<LocationPoint[]>([]);

  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const attemptRef = useRef(0);
  const unmountedRef = useRef(false);

  // connect/scheduleReconnect need to call each other; useRef avoids the
  // `Block-scoped variable used before its declaration` lint while keeping
  // the dependency-array semantics intact for `useEffect`.
  const connectRef = useRef<() => void>(() => {});
  const scheduleReconnectRef = useRef<() => void>(() => {});

  const buildWsUrl = useCallback((): string => {
    const token = localStorage.getItem(ACCESS_TOKEN_KEY) ?? '';
    const scheme = window.location.protocol === 'https:' ? 'wss' : 'ws';
    // Path must match nginx.conf /ws/ proxy_pass + api-gateway /ws/orders/{order_id} route.
    return `${scheme}://${window.location.host}/ws/orders/${orderId}?token=${encodeURIComponent(token)}`;
  }, [orderId]);

  const connect = useCallback((): void => {
    if (unmountedRef.current) return;
    setConnectionState(
      attemptRef.current === 0
        ? 'connecting'
        : attemptRef.current >= OFFLINE_THRESHOLD
          ? 'offline'
          : 'reconnecting',
    );
    const ws = new WebSocket(buildWsUrl());
    wsRef.current = ws;

    ws.onopen = (): void => {
      // NB: do NOT reset `attemptRef` here — backoff accumulates across closes
      // until an `online` event or a successful refresh-token path explicitly
      // resets it. This way 5+ flapping closes flip the chip to `offline`
      // even if each WS open() succeeded momentarily before the next close.
      setConnectionState('connected');
    };
    ws.onmessage = (e: MessageEvent): void => {
      try {
        const parsed = WSMessageSchema.parse(JSON.parse(String(e.data)));
        dispatchMessage(parsed, { orderId, setLastLocation, setHistory, qc });
      } catch (err) {
        console.warn('[useOrderTracking] invalid envelope', err);
      }
    };
    ws.onclose = (e: CloseEvent): void => {
      wsRef.current = null;
      if (unmountedRef.current) return;
      if (e.code === 1000) {
        setConnectionState('closed');
        return;
      }
      // Auth-class close codes (D-11): refresh token, then reconnect with new token in URL.
      if (e.code === 1008 || e.code === 4401) {
        api
          .get('/auth/refresh')
          .then(() => {
            attemptRef.current = 0;
            scheduleReconnectRef.current();
          })
          .catch(() => {
            // refresh interceptor handles /login redirect via auth:logout CustomEvent
            setConnectionState('closed');
          });
        return;
      }
      scheduleReconnectRef.current();
    };
    ws.onerror = (): void => {
      // onclose follows — no action needed here.
    };
  }, [buildWsUrl, qc, orderId]);

  const scheduleReconnect = useCallback((): void => {
    if (unmountedRef.current) return;
    const idx = Math.min(attemptRef.current, WS_BACKOFF_DELAYS_MS.length - 1);
    const base = WS_BACKOFF_DELAYS_MS[idx];
    const jitter = base * WS_BACKOFF_JITTER * (Math.random() * 2 - 1);
    const delay = Math.max(0, Math.round(base + jitter));
    attemptRef.current += 1;
    setConnectionState(
      attemptRef.current >= OFFLINE_THRESHOLD ? 'offline' : 'reconnecting',
    );
    reconnectTimerRef.current = setTimeout(() => connectRef.current(), delay);
  }, []);

  // Keep refs in sync with the latest callback identities (post-render so the
  // react-hooks lint rule doesn't flag ref-mutation in render).
  useEffect(() => {
    connectRef.current = connect;
    scheduleReconnectRef.current = scheduleReconnect;
  }, [connect, scheduleReconnect]);

  useEffect(() => {
    unmountedRef.current = false;
    connect();

    const onOnline = (): void => {
      if (wsRef.current?.readyState === WebSocket.OPEN) return;
      if (reconnectTimerRef.current) clearTimeout(reconnectTimerRef.current);
      attemptRef.current = 0;
      connect();
    };
    const onOffline = (): void => setConnectionState('offline');

    window.addEventListener('online', onOnline);
    window.addEventListener('offline', onOffline);

    return () => {
      unmountedRef.current = true;
      window.removeEventListener('online', onOnline);
      window.removeEventListener('offline', onOffline);
      if (reconnectTimerRef.current) clearTimeout(reconnectTimerRef.current);
      const ws = wsRef.current;
      if (ws && ws.readyState === WebSocket.OPEN) ws.close(1000, 'unmount');
      wsRef.current = null;
    };
  }, [connect]);

  return { connectionState, lastLocation, history };
}
