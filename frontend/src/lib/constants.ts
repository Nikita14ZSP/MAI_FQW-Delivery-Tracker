export const ROLES = { USER: 'user', COURIER: 'courier' } as const;
export type Role = (typeof ROLES)[keyof typeof ROLES];

// Geo bounding box for Moscow region (per 08-RESEARCH §3 reverse trigger).
// Order of fields chosen to match Nominatim viewbox semantics indirectly via lib/api/nominatim.ts.
export const MOSCOW_REGION_BBOX = {
  latMin: 54.25,
  latMax: 56.96,
  lonMin: 35.15,
  lonMax: 40.2,
} as const;

export function isInMoscowRegion(lat: number, lon: number): boolean {
  return (
    lat >= MOSCOW_REGION_BBOX.latMin &&
    lat <= MOSCOW_REGION_BBOX.latMax &&
    lon >= MOSCOW_REGION_BBOX.lonMin &&
    lon <= MOSCOW_REGION_BBOX.lonMax
  );
}

export const ORDER_STATUS_LABELS: Record<string, string> = {
  created: 'Создан',
  confirmed: 'Подтверждён',
  assigned: 'Назначен курьеру',
  picked_up: 'Забран',
  in_transit: 'В пути',
  delivered: 'Доставлен',
  cancelled: 'Отменён',
  failed: 'Сбой доставки',
  returned: 'Возвращён',
};

// Per 08-UI-SPEC §Color §Семантические цвета — `returned` is YELLOW (warning), not red.
export const ORDER_STATUS_COLORS: Record<string, string> = {
  created: 'bg-gray-100 text-gray-600 border-gray-200',
  confirmed: 'bg-blue-50 text-blue-700 border-blue-200',
  assigned: 'bg-orange-50 text-orange-700 border-orange-200',
  picked_up: 'bg-orange-50 text-orange-700 border-orange-200',
  in_transit: 'bg-orange-50 text-orange-700 border-orange-200',
  delivered: 'bg-green-50 text-green-700 border-green-200',
  cancelled: 'bg-red-50 text-red-700 border-red-200',
  failed: 'bg-red-50 text-red-700 border-red-200',
  returned: 'bg-yellow-50 text-yellow-800 border-yellow-200',
};

export const ACTIVE_STATUSES = [
  'created',
  'confirmed',
  'assigned',
  'picked_up',
  'in_transit',
] as const;

export const COMPLETED_STATUSES = [
  'delivered',
  'cancelled',
  'failed',
  'returned',
] as const;

export const CANCELLABLE_STATUSES = ['created', 'confirmed'] as const;

export const STEPPER_STATUSES = [
  'created',
  'confirmed',
  'assigned',
  'picked_up',
  'in_transit',
  'delivered',
] as const;

export type OrderStatus =
  | (typeof ACTIVE_STATUSES)[number]
  | (typeof COMPLETED_STATUSES)[number];

// --- Phase 9: WebSocket reconnect strategy (per 09-CONTEXT.md D-08, D-33) ---

/** Exponential backoff delays for WS reconnect (ms). Last value (30s) repeats indefinitely. */
export const WS_BACKOFF_DELAYS_MS = [1000, 2000, 4000, 8000, 16000, 30000] as const;

/** Jitter factor applied to each backoff delay (±25% per RFC 6455 best practice). */
export const WS_BACKOFF_JITTER = 0.25;

/** Terminal statuses — tracking page redirects on these. Per 09-CONTEXT.md D-29/D-30. */
export const FINAL_STATUSES = ['delivered', 'cancelled', 'failed', 'returned'] as const;

/** Statuses where tracking page is accessible (non-terminal). Per D-03. */
export const TRACKABLE_STATUSES = ['created', 'confirmed', 'assigned', 'picked_up', 'in_transit'] as const;

/** Statuses where courier is relevant (assigned and beyond). Per D-28. */
export const ASSIGNED_STATUSES = ['assigned', 'picked_up', 'in_transit'] as const;
