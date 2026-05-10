import { Fragment, type ComponentType } from 'react';
import {
  Check,
  Clock,
  CheckCircle,
  UserCheck,
  Package,
  Truck,
  CheckCircle2,
} from 'lucide-react';
import {
  ORDER_STATUS_LABELS,
  STEPPER_STATUSES,
  type OrderStatus,
} from '@/lib/constants';
import { cn } from '@/lib/utils';

type StepperStatus = (typeof STEPPER_STATUSES)[number];

const STEP_ICONS: Record<
  StepperStatus,
  ComponentType<{ className?: string }>
> = {
  created: Clock,
  confirmed: CheckCircle,
  assigned: UserCheck,
  picked_up: Package,
  in_transit: Truck,
  delivered: CheckCircle2,
};

export interface StatusStepperProps {
  currentStatus: OrderStatus;
}

export function StatusStepper({ currentStatus }: StatusStepperProps) {
  const idx = STEPPER_STATUSES.indexOf(currentStatus as StepperStatus);
  if (idx === -1) return null;

  return (
    <div
      className="w-full overflow-x-auto"
      role="list"
      aria-label="Прогресс заказа"
    >
      <ol className="flex min-w-[560px] items-start gap-0 pr-2">
        {STEPPER_STATUSES.map((status, i) => {
          const past = i < idx;
          const current = i === idx;
          const Icon = STEP_ICONS[status];
          const label = ORDER_STATUS_LABELS[status];
          const stateLabel = past
            ? 'пройден'
            : current
              ? 'текущий'
              : 'предстоит';
          return (
            <Fragment key={status}>
              <li
                role="listitem"
                className="flex flex-col items-center gap-2"
                aria-label={`${label}: ${stateLabel}`}
              >
                <div
                  className={cn(
                    'flex h-8 w-8 items-center justify-center rounded-full border-2',
                    past && 'bg-blue-600 border-blue-600',
                    current &&
                      'bg-white border-blue-600 ring-4 ring-blue-600/20 animate-pulse',
                    !past && !current && 'bg-white border-gray-200',
                  )}
                >
                  {past ? (
                    <Check className="h-3.5 w-3.5 text-white" />
                  ) : (
                    <Icon
                      className={cn(
                        'h-3.5 w-3.5',
                        current ? 'text-blue-600' : 'text-gray-300',
                      )}
                    />
                  )}
                </div>
                <span
                  className={cn(
                    'flex h-8 w-[84px] items-start justify-center text-center text-[11px] leading-[1.1] break-words',
                    past || current
                      ? 'text-gray-900 font-semibold'
                      : 'text-gray-400 font-normal',
                  )}
                >
                  {label}
                </span>
              </li>
              {i < STEPPER_STATUSES.length - 1 && (
                <span
                  className={cn(
                    'flex-1 h-0.5 mt-4',
                    i < idx - 1
                      ? 'bg-blue-600'
                      : i === idx - 1
                        ? 'bg-gradient-to-r from-blue-600 to-gray-200'
                        : 'bg-gray-200',
                  )}
                />
              )}
            </Fragment>
          );
        })}
      </ol>
    </div>
  );
}
