import { useState } from 'react';
import { Loader2, XCircle } from 'lucide-react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import {
  AlertDialog,
  AlertDialogTrigger,
  AlertDialogContent,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogAction,
  AlertDialogCancel,
} from '@/components/ui/alert-dialog';
import { Button } from '@/components/ui/button';
import { useToast } from '@/components/ui/use-toast';
import { cancelOrder } from '@/lib/api/orders';
import type { Order } from '@/types/order';

export interface CancelOrderDialogProps {
  orderId: string;
  userId: string;
  disabled?: boolean;
}

interface RollbackContext {
  listSnap: Order[] | undefined;
  detailSnap: Order | undefined;
}

export function CancelOrderDialog({
  orderId,
  userId,
  disabled,
}: CancelOrderDialogProps) {
  const qc = useQueryClient();
  const { toast } = useToast();
  const [open, setOpen] = useState(false);

  const mutation = useMutation<Order, unknown, void, RollbackContext>({
    mutationFn: () => cancelOrder(orderId),
    onMutate: async () => {
      await qc.cancelQueries({ queryKey: ['orders', userId] });
      await qc.cancelQueries({ queryKey: ['order', orderId] });
      const listSnap = qc.getQueryData<Order[]>(['orders', userId]);
      const detailSnap = qc.getQueryData<Order>(['order', orderId]);
      qc.setQueryData<Order[]>(['orders', userId], (old) =>
        (old ?? []).map((o) =>
          o.id === orderId ? { ...o, status: 'cancelled' } : o,
        ),
      );
      qc.setQueryData<Order>(['order', orderId], (old) =>
        old ? { ...old, status: 'cancelled' } : old,
      );
      return { listSnap, detailSnap };
    },
    onError: (err: unknown, _vars, ctx) => {
      if (ctx?.listSnap !== undefined) {
        qc.setQueryData(['orders', userId], ctx.listSnap);
      }
      if (ctx?.detailSnap !== undefined) {
        qc.setQueryData(['order', orderId], ctx.detailSnap);
      }
      const status = (err as { response?: { status?: number } })?.response
        ?.status;
      toast({
        variant: 'destructive',
        title:
          status === 409
            ? 'Заказ уже принят курьером, отмена невозможна'
            : 'Ошибка при отмене заказа',
      });
      qc.invalidateQueries({ queryKey: ['order', orderId] });
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['orders', userId] });
      qc.invalidateQueries({ queryKey: ['order', orderId] });
      toast({ title: 'Заказ отменён' });
      setOpen(false);
    },
  });

  return (
    <AlertDialog open={open} onOpenChange={setOpen}>
      <AlertDialogTrigger asChild>
        <Button
          type="button"
          variant="outline"
          disabled={disabled}
          className="mt-6 w-full border-red-300 text-red-600 hover:bg-red-50 hover:border-red-400"
          aria-label={`Отменить заказ #${orderId.slice(0, 8)}`}
        >
          <XCircle className="mr-2 h-4 w-4" />
          Отменить заказ
        </Button>
      </AlertDialogTrigger>
      <AlertDialogContent className="max-w-sm">
        <AlertDialogHeader>
          <AlertDialogTitle>Отменить заказ?</AlertDialogTitle>
          <AlertDialogDescription>
            Заказ будет отменён. Это действие нельзя отменить.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter className="flex flex-row-reverse gap-2">
          <AlertDialogAction
            disabled={mutation.isPending}
            className="bg-red-600 text-white hover:bg-red-700"
            onClick={(e) => {
              e.preventDefault();
              mutation.mutate();
            }}
          >
            {mutation.isPending ? (
              <>
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                Отменяю...
              </>
            ) : (
              'Да, отменить'
            )}
          </AlertDialogAction>
          <AlertDialogCancel>Не отменять</AlertDialogCancel>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
