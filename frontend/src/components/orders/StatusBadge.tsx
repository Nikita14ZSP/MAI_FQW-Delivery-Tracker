import type { ComponentType } from 'react';
import {
  AlertCircle,
  CheckCircle,
  CheckCircle2,
  Clock,
  Package,
  RotateCcw,
  Truck,
  UserCheck,
  XCircle,
} from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import {
  ORDER_STATUS_COLORS,
  ORDER_STATUS_LABELS,
  type OrderStatus,
} from '@/lib/constants';
import { cn } from '@/lib/utils';

const ICONS: Record<OrderStatus, ComponentType<{ className?: string }>> = {
  created: Clock,
  confirmed: CheckCircle,
  assigned: UserCheck,
  picked_up: Package,
  in_transit: Truck,
  delivered: CheckCircle2,
  cancelled: XCircle,
  failed: AlertCircle,
  returned: RotateCcw,
};

export interface StatusBadgeProps {
  status: OrderStatus;
  className?: string;
}

export function StatusBadge({ status, className }: StatusBadgeProps) {
  const Icon = ICONS[status] ?? Clock;
  const label = ORDER_STATUS_LABELS[status] ?? status;
  return (
    <Badge
      variant="outline"
      aria-label={label}
      className={cn(
        'inline-flex items-center gap-1',
        ORDER_STATUS_COLORS[status],
        className,
      )}
    >
      <Icon className="h-3 w-3" aria-hidden="true" />
      <span>{label}</span>
    </Badge>
  );
}
