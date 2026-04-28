import { RefreshCw } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';

export interface RefreshButtonProps {
  onClick: () => void;
  isFetching: boolean;
}

export function RefreshButton({ onClick, isFetching }: RefreshButtonProps) {
  return (
    <Button
      type="button"
      variant="ghost"
      size="icon"
      aria-label="Обновить список заказов"
      onClick={onClick}
    >
      <RefreshCw className={cn('h-4 w-4', isFetching && 'animate-spin')} />
    </Button>
  );
}
