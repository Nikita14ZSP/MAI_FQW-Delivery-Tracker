/**
 * CourierDashboardPage — single protected /courier page (D-04).
 *
 * Layout:
 *   CourierProfileHeader   (top — name, rating, status toggle)
 *   Tabs
 *     «Доступные заказы» → <AvailableOrdersTab isOnline={isOnline} />
 *     «Мои доставки»     → <MyDeliveriesTab    isOnline={isOnline} />
 *
 * Online state is lifted to this page (single useCourierStatus instance).
 * The status toggle in the header drives isOnline for both tabs (D-02/D-06).
 * CourierProfileHeader receives the lifted state via the optional statusControl prop
 * (added in 10-05 to CourierProfileHeader; backward-compatible when prop is absent).
 */

import { useAuth } from '@/hooks/useAuth';
import { useCourierStatus } from '@/hooks/useCourierStatus';
import { CourierProfileHeader } from '@/components/courier/CourierProfileHeader';
import { AvailableOrdersTab } from '@/components/courier/AvailableOrdersTab';
import { MyDeliveriesTab } from '@/components/courier/MyDeliveriesTab';
import {
  Tabs,
  TabsList,
  TabsTrigger,
  TabsContent,
} from '@/components/ui/tabs';

export function CourierDashboardPage() {
  const { user } = useAuth();

  // Lift the single online/offline state — one instance drives both tabs + header toggle.
  const { isOnline, toggle, isPending, status } = useCourierStatus({
    initialStatus: 'offline',
  });

  // Guard: don't render until auth is resolved.
  if (!user) return null;

  const courierId = user.id;

  return (
    <div className="mx-auto max-w-2xl px-4 py-6 space-y-4">
      <CourierProfileHeader
        courierId={courierId}
        initialStatus={status}
        statusControl={{ isOnline, toggle, isPending }}
      />

      <Tabs defaultValue="available">
        <TabsList className="grid w-full grid-cols-2">
          <TabsTrigger value="available">Доступные заказы</TabsTrigger>
          <TabsTrigger value="deliveries">Мои доставки</TabsTrigger>
        </TabsList>

        <TabsContent value="available">
          <AvailableOrdersTab isOnline={isOnline} />
        </TabsContent>

        <TabsContent value="deliveries">
          <MyDeliveriesTab isOnline={isOnline} />
        </TabsContent>
      </Tabs>
    </div>
  );
}
