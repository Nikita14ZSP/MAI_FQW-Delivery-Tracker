import { z } from 'zod';
import { ROLES } from '@/lib/constants';

export const RegisterSchema = z.object({
  role: z.enum([ROLES.USER, ROLES.COURIER], { message: 'Выберите роль' }),
  email: z.string().min(1, 'Введите email').email('Некорректный email'),
  password: z
    .string()
    .min(1, 'Введите пароль')
    .min(8, 'Пароль должен содержать не менее 8 символов'),
  first_name: z.string().min(1, 'Введите имя').max(100),
  last_name: z.string().min(1, 'Введите фамилию').max(100),
  phone: z
    .string()
    .regex(/^\+7\d{10}$/, 'Введите телефон в формате +7XXXXXXXXXX')
    .optional()
    .or(z.literal('')),
});

export const LoginSchema = z.object({
  email: z.string().min(1, 'Введите email').email('Некорректный email'),
  password: z.string().min(1, 'Введите пароль'),
});

export type RegisterInput = z.infer<typeof RegisterSchema>;
export type LoginInput = z.infer<typeof LoginSchema>;
