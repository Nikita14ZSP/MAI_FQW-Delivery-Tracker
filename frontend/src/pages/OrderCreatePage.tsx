import { useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useForm, useFieldArray, useWatch } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { useAuth } from '@/hooks/useAuth';
import { useToast } from '@/components/ui/use-toast';
import {
  Form,
  FormField,
  FormItem,
  FormLabel,
  FormControl,
  FormMessage,
} from '@/components/ui/form';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Label } from '@/components/ui/label';
import { ArrowLeft } from 'lucide-react';

import { CreateOrderSchema, type CreateOrderInput } from '@/lib/schemas/order';
import { createOrder } from '@/lib/api/orders';
import { listCategories } from '@/lib/api/menu';
import { isInMoscowRegion } from '@/lib/constants';

import { AddressSearchInput } from '@/components/orders/AddressSearchInput';
import {
  MapPicker,
  type MapPickerPosition,
} from '@/components/orders/MapPicker';
import { OutOfZoneWarning } from '@/components/orders/OutOfZoneWarning';
import { CategoryTabs } from '@/components/orders/CategoryTabs';
import { MenuGrid } from '@/components/orders/MenuGrid';
import { CartPanel } from '@/components/orders/CartPanel';
import { PhoneInput } from '@/components/orders/PhoneInput';
import { PaymentMethodRadio } from '@/components/orders/PaymentMethodRadio';
import type { MenuItem } from '@/types/order';

export function OrderCreatePage() {
  const { user } = useAuth();
  const { toast } = useToast();
  const navigate = useNavigate();
  const qc = useQueryClient();

  const form = useForm<CreateOrderInput>({
    resolver: zodResolver(CreateOrderSchema),
    defaultValues: {
      delivery_address: '',
      delivery_lat: undefined as unknown as number,
      delivery_lng: undefined as unknown as number,
      items: [],
      contact_phone: user?.phone ?? '',
      payment_method: 'card_on_delivery',
    },
  });

  const { fields, append, remove, update } = useFieldArray({
    control: form.control,
    name: 'items',
  });
  const watchedItemsRaw = useWatch({ control: form.control, name: 'items' });
  const watchedItems = useMemo(
    () => watchedItemsRaw ?? [],
    [watchedItemsRaw],
  );
  const total = useMemo(
    () => watchedItems.reduce((s, it) => s + it.price * it.quantity, 0),
    [watchedItems],
  );

  // Map position state lifted to page (so AddressSearchInput + MapPicker share)
  const [position, setPosition] = useState<MapPickerPosition | null>(null);
  const outOfZone = position
    ? !isInMoscowRegion(position.lat, position.lon)
    : false;

  // Sync map position into form state
  useEffect(() => {
    if (position) {
      form.setValue('delivery_lat', position.lat, { shouldValidate: true });
      form.setValue('delivery_lng', position.lon, { shouldValidate: true });
      if (position.displayName)
        form.setValue('delivery_address', position.displayName, {
          shouldValidate: true,
        });
    }
  }, [position, form]);

  // Categories
  const { data: categories = [] } = useQuery({
    queryKey: ['menu', 'categories'],
    queryFn: listCategories,
    staleTime: Infinity,
    gcTime: Infinity,
  });
  const [activeCategoryId, setActiveCategoryId] = useState<string>('');
  useEffect(() => {
    if (!activeCategoryId && categories.length > 0)
      setActiveCategoryId(categories[0].id);
  }, [categories, activeCategoryId]);

  // Cart manipulation — keyed by MenuItem.name (matches backend CreateOrderRequest.items[].name)
  const handleIncrement = (item: MenuItem) => {
    const idx = fields.findIndex((f) => f.name === item.name);
    if (idx >= 0) {
      update(idx, {
        ...fields[idx],
        quantity: fields[idx].quantity + 1,
      });
    } else {
      append({ name: item.name, quantity: 1, price: Number(item.price) });
    }
  };
  const handleDecrement = (item: MenuItem) => {
    const idx = fields.findIndex((f) => f.name === item.name);
    if (idx < 0) return;
    if (fields[idx].quantity === 1) remove(idx);
    else
      update(idx, {
        ...fields[idx],
        quantity: fields[idx].quantity - 1,
      });
  };

  // Mutation
  const mutation = useMutation({
    mutationFn: (input: CreateOrderInput) =>
      createOrder({
        user_id: user!.id,
        delivery_address: input.delivery_address,
        delivery_coordinates: {
          latitude: input.delivery_lat,
          longitude: input.delivery_lng,
        },
        items: input.items,
        contact_phone: input.contact_phone,
        payment_method: input.payment_method,
      }),
    onSuccess: (order) => {
      qc.invalidateQueries({ queryKey: ['orders', user!.id] });
      toast({ title: 'Заказ успешно создан' });
      navigate(`/orders/${order.id}`);
    },
    onError: () => {
      toast({
        variant: 'destructive',
        title: 'Ошибка при создании заказа. Попробуйте ещё раз',
      });
    },
  });

  const submitDisabled =
    watchedItems.length === 0 || position === null || outOfZone;

  const onSubmit = form.handleSubmit(
    (data) => {
      if (outOfZone) {
        toast({
          variant: 'destructive',
          title: 'Доставка доступна только по Москве и Московской области',
        });
        return;
      }
      mutation.mutate(data);
    },
    () => {
      toast({
        variant: 'destructive',
        title: 'Проверьте правильность заполнения формы',
      });
    },
  );

  // For getQty: build a map name -> qty from watchedItems for MenuGrid
  const qtyByName = useMemo(() => {
    const m = new Map<string, number>();
    watchedItems.forEach((w) => m.set(w.name, w.quantity));
    return m;
  }, [watchedItems]);

  const contactBlock = (
    <div className="flex flex-col gap-3">
      <FormField
        control={form.control}
        name="contact_phone"
        render={({ field }) => (
          <FormItem>
            <FormLabel>Телефон для связи</FormLabel>
            <FormControl>
              <PhoneInput {...field} />
            </FormControl>
            <FormMessage />
          </FormItem>
        )}
      />
      <FormField
        control={form.control}
        name="payment_method"
        render={({ field }) => (
          <FormItem>
            <FormLabel>Способ оплаты</FormLabel>
            <FormControl>
              <PaymentMethodRadio
                value={field.value}
                onChange={field.onChange}
              />
            </FormControl>
            <FormMessage />
          </FormItem>
        )}
      />
    </div>
  );

  return (
    <Form {...form}>
      <form
        noValidate
        onSubmit={onSubmit}
        className="min-h-screen bg-gray-50"
      >
        <div className="mx-auto max-w-6xl px-4 py-6">
          <header className="mb-6">
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="mb-2 -ml-2 h-8 px-2 text-gray-600 hover:text-gray-900"
              onClick={() => navigate('/orders')}
            >
              <ArrowLeft className="mr-1 h-4 w-4" />
              К списку заказов
            </Button>
            <h1 className="text-2xl font-semibold text-gray-900">
              Создать заказ
            </h1>
            <p className="text-sm text-gray-500">Выберите адрес и блюда</p>
          </header>
          <div className="lg:grid lg:grid-cols-[1fr_340px] lg:gap-6">
            <div className="flex flex-col gap-6">
              <Card className="p-4">
                <Label className="text-sm font-semibold">
                  Адрес доставки
                </Label>
                <div className="mt-2">
                  <AddressSearchInput
                    onSelect={(s) =>
                      setPosition({
                        lat: s.lat,
                        lon: s.lon,
                        displayName: s.displayName,
                      })
                    }
                  />
                </div>
                <div className="mt-3">
                  <MapPicker
                    position={position}
                    onPositionChange={setPosition}
                  />
                </div>
                <OutOfZoneWarning show={outOfZone} />
              </Card>
              <Card className="p-4">
                <Label className="mb-3 text-sm font-semibold">
                  Выберите блюда
                </Label>
                <CategoryTabs
                  categories={categories}
                  activeId={activeCategoryId}
                  onChange={setActiveCategoryId}
                />
                <MenuGrid
                  activeCategoryId={activeCategoryId}
                  getQty={(item) => qtyByName.get(item.name) ?? 0}
                  onIncrement={handleIncrement}
                  onDecrement={handleDecrement}
                />
                {/* Mobile-only contact card — sticky bottom doesn't host these */}
                <div className="mt-6 lg:hidden">{contactBlock}</div>
              </Card>
            </div>
            <div className="hidden lg:block">
              <CartPanel
                items={watchedItems}
                total={total}
                isSubmitting={mutation.isPending}
                submitDisabled={submitDisabled}
                onSubmit={onSubmit}
                variant="desktop"
                contactSlot={contactBlock}
              />
            </div>
            {/* Mobile sticky bottom */}
            <div className="lg:hidden pb-24">
              <CartPanel
                items={watchedItems}
                total={total}
                isSubmitting={mutation.isPending}
                submitDisabled={submitDisabled}
                onSubmit={onSubmit}
                variant="mobile"
              />
            </div>
          </div>
        </div>
      </form>
    </Form>
  );
}
