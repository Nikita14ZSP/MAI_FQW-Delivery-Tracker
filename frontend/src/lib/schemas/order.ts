import { z } from 'zod';

export const OrderItemSchema = z.object({
  name: z.string().min(1),
  quantity: z.number().int().min(1),
  price: z.number().positive(),
});

export const CreateOrderSchema = z.object({
  delivery_address: z.string().min(1, 'Выберите адрес на карте'),
  delivery_lat: z.number({ required_error: 'Выберите адрес на карте' }),
  delivery_lng: z.number({ required_error: 'Выберите адрес на карте' }),
  items: z.array(OrderItemSchema).min(1, 'Добавьте хотя бы одно блюдо'),
  contact_phone: z.preprocess(
    (val) =>
      typeof val === 'string' ? val.replace(/[\s\-().]/g, '') : val,
    z.string().regex(/^\+7\d{10}$/, 'Введите телефон в формате +7XXXXXXXXXX'),
  ),
  payment_method: z.enum(['cash', 'card_on_delivery', 'online'], {
    required_error: 'Выберите способ оплаты',
  }),
});

export type CreateOrderInput = z.infer<typeof CreateOrderSchema>;
export type OrderItemInput = z.infer<typeof OrderItemSchema>;
