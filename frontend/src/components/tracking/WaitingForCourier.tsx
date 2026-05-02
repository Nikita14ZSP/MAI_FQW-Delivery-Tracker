import { Loader2 } from 'lucide-react';

interface WaitingForCourierProps {
  className?: string;
}

export function WaitingForCourier({ className }: WaitingForCourierProps) {
  return (
    <div
      role="status"
      aria-live="polite"
      className={`flex items-center gap-3 py-4 ${className ?? ''}`}
    >
      <Loader2 className="h-5 w-5 animate-spin text-orange-500" aria-hidden="true" />
      <span className="text-sm text-gray-700">Поиск курьера...</span>
    </div>
  );
}
