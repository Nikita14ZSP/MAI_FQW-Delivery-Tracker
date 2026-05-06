import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Star, Loader2 } from 'lucide-react';
import { Card } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import {
  AlertDialog,
  AlertDialogTrigger,
  AlertDialogContent,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogFooter,
  AlertDialogCancel,
} from '@/components/ui/alert-dialog';
import { useAuth } from '@/hooks/useAuth';
import { useCourierStatus } from '@/hooks/useCourierStatus';
import { getCourierRating } from '@/lib/api/courier';

/** Optional lifted-state control passed from CourierDashboardPage (Plan 10-05).
 *  When provided, the page owns the single useCourierStatus instance and passes
 *  its state down here. When absent, CourierProfileHeader manages its own hook
 *  (preserves backward-compatible behaviour for 10-02 standalone tests). */
export interface StatusControl {
  isOnline: boolean;
  toggle: () => void;
  isPending: boolean;
}

export interface CourierProfileHeaderProps {
  courierId: string;
  /** Initial online/offline state from the parent page (defaults to 'offline' if unknown). */
  initialStatus: string;
  /** Lifted status control from the parent page. When provided, replaces the internal hook. */
  statusControl?: StatusControl;
}

/**
 * CourierProfileHeader — always-visible top section of the courier dashboard.
 *
 * Shows:
 * - Courier display name (from useAuth; fallback 'Курьер')
 * - Average rating + total reviews (from GET /v1/couriers/{id}/rating)
 * - «Смотреть отзывы» button (visible only when total_ratings > 0)
 *   that opens a Radix AlertDialog modal with ALL reviews newest-first
 * - Online/offline status toggle button (powered by useCourierStatus)
 *
 * Design: minimalist Card, Tailwind v4 CSS vars, consistent with Phase 8+ style.
 */
export function CourierProfileHeader({
  courierId,
  initialStatus,
  statusControl,
}: CourierProfileHeaderProps) {
  const { user } = useAuth();
  const name = [user?.first_name, user?.last_name].filter(Boolean).join(' ') || 'Курьер';

  const { data: rating, isLoading } = useQuery({
    queryKey: ['courier-rating', courierId],
    queryFn: () => getCourierRating(courierId),
    enabled: !!courierId,
  });

  // If the parent page has lifted the online state (Plan 10-05), use that.
  // Otherwise fall back to the local hook (backward-compatible for standalone use).
  const internal = useCourierStatus({ initialStatus });
  const { isOnline, toggle, isPending } = statusControl ?? internal;

  // Sort all ratings newest-first (no slice — modal shows ALL entries, D-06)
  const allReviews = rating?.ratings
    .slice()
    .sort((a, b) => (a.created_at < b.created_at ? 1 : -1));

  // Modal open state for reviews (D-07 controlled AlertDialog pattern)
  const [reviewsOpen, setReviewsOpen] = useState(false);

  return (
    <Card className="p-4">
      {/* Row 1: name + status toggle */}
      <div className="flex items-center justify-between gap-4">
        <h2 className="text-lg font-semibold text-[var(--card-foreground)]">{name}</h2>
        <Button
          onClick={toggle}
          disabled={isPending}
          aria-pressed={isOnline}
          className={
            isOnline
              ? 'bg-green-600 hover:bg-green-700 text-white'
              : 'bg-gray-200 text-gray-700 hover:bg-gray-300'
          }
          size="sm"
        >
          {isPending ? (
            <Loader2 className="h-4 w-4 animate-spin" />
          ) : isOnline ? (
            'Работаю'
          ) : (
            'Не работаю'
          )}
        </Button>
      </div>

      {/* Row 2: rating */}
      <div className="mt-2">
        {isLoading ? (
          <div className="h-4 w-32 animate-pulse rounded bg-gray-200" />
        ) : rating && rating.total_ratings > 0 ? (
          <span className="text-sm text-[var(--card-foreground)]">
            {rating.average_stars.toFixed(1)}{' '}
            <Star className="inline h-4 w-4 fill-yellow-400 text-yellow-400" />{' '}
            · {rating.total_ratings} отзывов
          </span>
        ) : (
          <span className="text-sm text-gray-500">
            Рейтинг появится после первой доставки
          </span>
        )}
      </div>

      {/* «Смотреть отзывы» button + modal — visible only when total_ratings > 0 (D-05) */}
      {rating && rating.total_ratings > 0 && (
        <div className="mt-3">
          <AlertDialog open={reviewsOpen} onOpenChange={setReviewsOpen}>
            <AlertDialogTrigger asChild>
              <Button variant="outline" size="sm">Смотреть отзывы</Button>
            </AlertDialogTrigger>
            <AlertDialogContent className="top-[6vh] translate-y-0 max-h-[85vh] overflow-y-auto">
              <AlertDialogHeader>
                <AlertDialogTitle>Отзывы</AlertDialogTitle>
              </AlertDialogHeader>
              <div className="overflow-y-auto max-h-[65vh] space-y-2">
                {allReviews?.map((r) => (
                  <div
                    key={r.delivery_id}
                    className="text-sm text-gray-600 border-b border-[var(--border)] pb-2 last:border-0"
                  >
                    <div>
                      {'★'.repeat(r.stars)}{' '}
                      <span className="text-gray-400">
                        {new Date(r.created_at).toLocaleDateString('ru-RU')}
                      </span>
                    </div>
                    <div>{r.comment || '(без комментария)'}</div>
                  </div>
                ))}
              </div>
              <AlertDialogFooter>
                <AlertDialogCancel>Закрыть</AlertDialogCancel>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
        </div>
      )}
    </Card>
  );
}
