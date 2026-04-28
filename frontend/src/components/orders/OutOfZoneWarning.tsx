import { AlertTriangle } from 'lucide-react';

export interface OutOfZoneWarningProps {
  show: boolean;
}

export function OutOfZoneWarning({ show }: OutOfZoneWarningProps) {
  if (!show) return null;
  return (
    <div
      className="mt-2 flex items-center gap-2 text-sm text-red-600"
      role="alert"
    >
      <AlertTriangle
        className="h-3.5 w-3.5 flex-shrink-0"
        aria-hidden="true"
      />
      <span>Доставка доступна только по Москве и МО</span>
    </div>
  );
}
