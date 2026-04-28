import type { ComponentType } from 'react';
import { AlertCircle, RotateCcw, XCircle } from 'lucide-react';

type FinalStatus = 'cancelled' | 'failed' | 'returned';

interface BannerCopy {
  title: string;
  body: string;
  Icon: ComponentType<{ className?: string; 'aria-hidden'?: boolean }>;
  box: string;
  titleClass: string;
  iconClass: string;
}

const COPY: Record<FinalStatus, BannerCopy> = {
  cancelled: {
    title: 'Заказ отменён',
    body: 'Вы отменили этот заказ',
    Icon: XCircle,
    box: 'bg-red-50 border border-red-100',
    titleClass: 'text-red-700',
    iconClass: 'text-red-500',
  },
  failed: {
    title: 'Доставка не состоялась',
    body: 'Курьер не смог выполнить доставку',
    Icon: AlertCircle,
    box: 'bg-red-50 border border-red-100',
    titleClass: 'text-red-700',
    iconClass: 'text-red-500',
  },
  returned: {
    title: 'Заказ возвращён',
    body: 'Заказ был возвращён отправителю',
    Icon: RotateCcw,
    box: 'bg-yellow-50 border border-yellow-200',
    titleClass: 'text-yellow-800',
    iconClass: 'text-yellow-600',
  },
};

export interface StatusBannerProps {
  status: FinalStatus;
}

export function StatusBanner({ status }: StatusBannerProps) {
  const c = COPY[status];
  const Icon = c.Icon;
  return (
    <div
      className={`flex items-start gap-3 rounded-xl p-4 ${c.box}`}
      role="status"
    >
      <Icon className={`h-6 w-6 ${c.iconClass}`} aria-hidden={true} />
      <div className="flex flex-col gap-2">
        <span className={`text-sm font-semibold ${c.titleClass}`}>
          {c.title}
        </span>
        <span className="text-xs text-gray-500">{c.body}</span>
      </div>
    </div>
  );
}
