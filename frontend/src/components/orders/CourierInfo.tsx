import { Clock } from 'lucide-react';

export interface CourierInfoProps {
  courierName?: string | null;
  etaMinutes?: number | null;
  averageStars?: number | null;
}

export function CourierInfo({
  courierName,
  etaMinutes,
  averageStars,
}: CourierInfoProps) {
  if (!courierName && (etaMinutes === null || etaMinutes === undefined)) {
    return null;
  }
  return (
    <div>
      <p className="text-sm font-semibold uppercase tracking-wide text-gray-500">
        Курьер
      </p>
      {courierName && (
        <p className="mt-1 text-base text-gray-900">
          {courierName}
          {averageStars != null && (
            <span className="ml-2 text-sm text-amber-500">
              ★ {averageStars.toFixed(1)}
            </span>
          )}
        </p>
      )}
      {etaMinutes != null && (
        <p className="mt-1 text-sm text-gray-500">
          <Clock className="mr-1 inline h-3.5 w-3.5" aria-hidden={true} />
          Ожидаемое время доставки: ~{etaMinutes} мин
        </p>
      )}
    </div>
  );
}
