import { useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { Star } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { useToast } from '@/components/ui/use-toast';
import { submitRating } from '@/lib/api/ratings';
import { cn } from '@/lib/utils';

export interface RatingCardProps {
  orderId: string;
  deliveryId: string;
}

interface SubmittedRating {
  stars: number;
  comment: string;
}

const submittedKey = (id: string) => `rating:submitted:${id}`;
const skippedKey = (id: string) => `rating:skipped:${id}`;

function readSubmitted(orderId: string): SubmittedRating | null {
  try {
    const raw = localStorage.getItem(submittedKey(orderId));
    if (!raw) return null;
    const parsed = JSON.parse(raw) as SubmittedRating;
    if (typeof parsed?.stars === 'number') return parsed;
    return null;
  } catch {
    return null;
  }
}

function StarRow({
  value,
  onPick,
}: {
  value: number;
  onPick?: (n: number) => void;
}) {
  return (
    <div className="flex gap-1">
      {[1, 2, 3, 4, 5].map((n) => {
        const filled = n <= value;
        const interactive = !!onPick;
        return (
          <button
            key={n}
            type="button"
            aria-label={`Оценить на ${n} звёзд`}
            aria-pressed={filled}
            disabled={!interactive}
            onClick={interactive ? () => onPick?.(n) : undefined}
            className={cn(
              'rounded p-0.5',
              interactive &&
                'transition-transform hover:scale-110 focus:outline-none focus-visible:ring-2 focus-visible:ring-orange-400',
              !interactive && 'cursor-default',
            )}
          >
            <Star
              className={cn(
                'h-7 w-7',
                filled
                  ? 'fill-yellow-400 text-yellow-400'
                  : 'text-gray-300',
              )}
            />
          </button>
        );
      })}
    </div>
  );
}

export function RatingCard({ orderId, deliveryId }: RatingCardProps) {
  // Read localStorage once on mount (lazy initializer — not every render).
  const [submitted, setSubmitted] = useState<SubmittedRating | null>(() =>
    readSubmitted(orderId),
  );
  const [skipped, setSkipped] = useState<boolean>(
    () => localStorage.getItem(skippedKey(orderId)) === '1',
  );
  const [stars, setStars] = useState(0);
  const [comment, setComment] = useState('');
  const { toast } = useToast();

  const mutation = useMutation({
    mutationFn: () => submitRating(deliveryId, stars, comment),
    onSuccess: () => {
      const record: SubmittedRating = { stars, comment };
      localStorage.setItem(submittedKey(orderId), JSON.stringify(record));
      setSubmitted(record);
      toast({ title: 'Спасибо за оценку' });
    },
    onError: () => {
      toast({
        variant: 'destructive',
        title: 'Не удалось отправить оценку. Попробуйте позже',
      });
    },
  });

  // D-04: form not shown after skip.
  if (skipped && !submitted) return null;

  // D-03: read-only block after submit (or if already submitted on a prior visit).
  if (submitted) {
    return (
      <Card className="border-orange-200 p-4">
        <p className="mb-2 text-sm font-semibold uppercase tracking-wide text-gray-500">
          Вы оценили курьера
        </p>
        <StarRow value={submitted.stars} />
        {submitted.comment.trim() !== '' && (
          <p className="mt-3 border-l-2 border-orange-200 pl-3 text-sm italic text-gray-600">
            {submitted.comment}
          </p>
        )}
      </Card>
    );
  }

  // D-01/D-02: rating form.
  return (
    <Card className="border-orange-200 p-4">
      <p className="mb-3 text-sm font-semibold uppercase tracking-wide text-gray-500">
        Оцените курьера
      </p>

      <StarRow value={stars} onPick={setStars} />

      <textarea
        value={comment}
        onChange={(e) => setComment(e.target.value)}
        placeholder="Ваш отзыв о курьере (необязательно)"
        rows={3}
        className="mt-4 w-full resize-none rounded-md border border-[var(--border)] bg-[var(--background)] px-3 py-2 text-sm text-gray-900 placeholder:text-gray-400 focus:outline-none focus-visible:ring-2 focus-visible:ring-orange-400"
      />

      <div className="mt-4 flex items-center gap-3">
        <Button
          type="button"
          className="bg-orange-500 text-white hover:bg-orange-600"
          disabled={stars === 0 || mutation.isPending}
          onClick={() => mutation.mutate()}
        >
          {mutation.isPending ? 'Отправляем…' : 'Отправить оценку'}
        </Button>
        <Button
          type="button"
          variant="ghost"
          disabled={mutation.isPending}
          onClick={() => {
            localStorage.setItem(skippedKey(orderId), '1');
            setSkipped(true);
          }}
        >
          Пропустить
        </Button>
      </div>
    </Card>
  );
}
