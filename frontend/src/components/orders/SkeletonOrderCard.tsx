import { Card } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';

export function SkeletonOrderCard() {
  return (
    <Card className="rounded-xl p-4">
      <div className="mb-2 flex items-center justify-between">
        <Skeleton className="h-5 w-24 rounded-full" />
        <Skeleton className="h-4 w-16" />
      </div>
      <Skeleton className="mt-2 h-4 w-3/4" />
      <Skeleton className="mt-2 h-3 w-1/2" />
      <div className="mt-2 flex justify-end">
        <Skeleton className="h-4 w-16" />
      </div>
    </Card>
  );
}
