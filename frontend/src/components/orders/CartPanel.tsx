import { Loader2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';

export interface CartItem {
  name: string;
  quantity: number;
  price: number;
}

export interface CartPanelProps {
  items: CartItem[];
  total: number;
  isSubmitting: boolean;
  submitDisabled: boolean;
  onSubmit: () => void;
  variant: 'desktop' | 'mobile';
  /** Slot for phone Input + payment radio (desktop only) */
  contactSlot?: React.ReactNode;
}

export function CartPanel({
  items,
  total,
  isSubmitting,
  submitDisabled,
  onSubmit,
  variant,
  contactSlot,
}: CartPanelProps) {
  if (variant === 'mobile') {
    return (
      <div className="fixed bottom-0 left-0 right-0 z-40 border-t border-gray-200 bg-white px-4 py-3 shadow-[0_-4px_12px_rgba(0,0,0,0.08)]">
        <div className="flex items-center justify-between gap-4">
          <div className="flex flex-col">
            <span className="text-xs text-gray-500">{items.length} блюд</span>
            <span className="text-base font-semibold text-gray-900">
              {total} ₽
            </span>
          </div>
          <Button
            type="button"
            onClick={onSubmit}
            disabled={submitDisabled || isSubmitting}
            className="flex-1 max-w-[200px]"
          >
            {isSubmitting ? (
              <>
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                Оформление...
              </>
            ) : (
              'Оформить заказ'
            )}
          </Button>
        </div>
      </div>
    );
  }

  return (
    <div className="sticky top-6 self-start">
      <Card className="rounded-xl p-4">
        <h3 className="mb-3 text-base font-semibold text-gray-900">
          Ваш заказ
        </h3>
        <div className="flex max-h-[300px] flex-col gap-2 overflow-y-auto">
          {items.length === 0 ? (
            <p className="text-sm text-gray-400">Корзина пуста</p>
          ) : (
            items.map((it, i) => (
              <div
                key={i}
                className="flex items-center justify-between text-sm"
              >
                <span className="text-gray-700">
                  {it.name} × {it.quantity}
                </span>
                <span className="font-semibold text-gray-900">
                  {it.price * it.quantity} ₽
                </span>
              </div>
            ))
          )}
        </div>
        <div className="my-3 border-t border-gray-100" />
        <div className="flex justify-between text-base font-semibold text-gray-900">
          <span>Итого</span>
          <span>{total} ₽</span>
        </div>
        {contactSlot && (
          <>
            <div className="my-3 border-t border-gray-100" />
            {contactSlot}
          </>
        )}
        <Button
          type="button"
          onClick={onSubmit}
          disabled={submitDisabled || isSubmitting}
          className="mt-4 w-full"
        >
          {isSubmitting ? (
            <>
              <Loader2 className="mr-2 h-4 w-4 animate-spin" />
              Оформление...
            </>
          ) : (
            'Оформить заказ'
          )}
        </Button>
      </Card>
    </div>
  );
}
