import { differenceInMinutes, parseISO } from 'date-fns';
import { useEffect, useState } from 'react';
import { Clock } from 'lucide-react';

interface EtaChipProps {
  etaIso: string | null;
  className?: string;
}

export function EtaChip({ etaIso, className }: EtaChipProps) {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    if (!etaIso) return;
    const id = setInterval(() => setNow(Date.now()), 60_000);
    return () => clearInterval(id);
  }, [etaIso]);
  if (!etaIso) return null;
  const minutes = differenceInMinutes(parseISO(etaIso), now);
  return (
    <div
      role="status"
      aria-label="Расчётное время прибытия"
      className={`inline-flex items-center gap-1 text-xs px-2 py-1 rounded-full bg-blue-50 text-blue-700 border border-blue-200 ${className ?? ''}`}
    >
      <Clock className="h-3 w-3" />
      {minutes <= 0 ? 'Прибыл' : `~${minutes} мин`}
    </div>
  );
}
