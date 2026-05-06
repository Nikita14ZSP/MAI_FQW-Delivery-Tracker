import { useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { updateCourierStatus } from '@/lib/api/courier';
import { useToast } from '@/components/ui/use-toast';

/**
 * useCourierStatus — manages the courier online/offline toggle state.
 *
 * - Calls updateCourierStatus(next) via useMutation.
 * - Status only flips on server-confirmed success (onSuccess), never optimistically.
 *   This is intentional: server is the source of truth for the 409 guard.
 *   Pre-flipping would cause a flash-of-wrong-state on 409 rollback.
 * - On HTTP 409 (active_delivery_exists): status stays unchanged + destructive toast.
 * - On any other error: generic destructive toast.
 */
export function useCourierStatus(opts: { initialStatus: string }): {
  status: 'available' | 'offline';
  isOnline: boolean;
  toggle: () => void;
  isPending: boolean;
} {
  const [status, setStatus] = useState<'available' | 'offline'>(
    opts.initialStatus === 'available' ? 'available' : 'offline',
  );
  const { toast } = useToast();

  const mutation = useMutation({
    mutationFn: (next: 'available' | 'offline') => updateCourierStatus(next),
    onSuccess: (data) => {
      setStatus(data.status === 'available' ? 'available' : 'offline');
    },
    onError: (err) => {
      const code = (err as { response?: { status?: number } })?.response?.status;
      toast({
        variant: 'destructive',
        title:
          code === 409
            ? 'Завершите активные доставки, чтобы уйти со смены'
            : 'Не удалось изменить статус',
      });
    },
  });

  const toggle = () => {
    mutation.mutate(status === 'available' ? 'offline' : 'available');
  };

  return {
    status,
    isOnline: status === 'available',
    toggle,
    isPending: mutation.isPending,
  };
}
