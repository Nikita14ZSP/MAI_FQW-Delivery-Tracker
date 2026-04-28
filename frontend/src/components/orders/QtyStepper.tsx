import { Minus, Plus } from 'lucide-react';
import { Button } from '@/components/ui/button';

export interface QtyStepperProps {
  value: number;
  onIncrement: () => void;
  onDecrement: () => void;
}

export function QtyStepper({
  value,
  onIncrement,
  onDecrement,
}: QtyStepperProps) {
  return (
    <div className="flex items-center gap-2">
      <Button
        type="button"
        variant="outline"
        size="icon"
        className="h-9 w-9 rounded-full disabled:opacity-40 disabled:cursor-not-allowed"
        disabled={value === 0}
        aria-label="Уменьшить количество"
        onClick={onDecrement}
      >
        <Minus className="h-3.5 w-3.5" />
      </Button>
      <span
        className="min-w-[20px] text-center text-sm font-semibold text-gray-900"
        aria-live="polite"
      >
        {value}
      </span>
      <Button
        type="button"
        variant="outline"
        size="icon"
        className="h-9 w-9 rounded-full"
        aria-label="Увеличить количество"
        onClick={onIncrement}
      >
        <Plus className="h-3.5 w-3.5" />
      </Button>
    </div>
  );
}
