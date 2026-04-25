import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { ShoppingBag, Bike, Loader2 } from 'lucide-react';
import { isAxiosError } from 'axios';
import { RegisterSchema, type RegisterInput } from '@/lib/schemas/auth';
import { ROLES } from '@/lib/constants';
import { useAuth } from '@/hooks/useAuth';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form';
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group';
import { useToast } from '@/components/ui/use-toast';

export interface RegisterFormProps {
  onSuccess?: () => void;
}

export function RegisterForm({ onSuccess }: RegisterFormProps) {
  const { register: doRegister } = useAuth();
  const { toast } = useToast();

  const form = useForm<RegisterInput>({
    resolver: zodResolver(RegisterSchema),
    defaultValues: {
      role: ROLES.USER,
      email: '',
      password: '',
      first_name: '',
      last_name: '',
      phone: '',
    },
    mode: 'onSubmit',
  });

  async function onSubmit(values: RegisterInput) {
    try {
      await doRegister(values);
      onSuccess?.();
    } catch (err) {
      if (isAxiosError(err) && err.response?.status === 409) {
        toast({
          variant: 'destructive',
          title: 'Пользователь с таким email уже зарегистрирован',
        });
        form.setError('email', {
          type: 'server',
          message: 'Email уже используется',
        });
        return;
      }
      toast({
        variant: 'destructive',
        title: 'Ошибка сервера. Попробуйте позже',
      });
    }
  }

  const selectedRole = form.watch('role');

  return (
    <Form {...form}>
      <form
        onSubmit={form.handleSubmit(onSubmit)}
        className="flex flex-col gap-4"
        aria-label="register form"
        noValidate
      >
        <FormField
          control={form.control}
          name="role"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Кто вы?</FormLabel>
              <FormControl>
                <RadioGroup
                  value={field.value}
                  onValueChange={field.onChange}
                  className="grid grid-cols-2 gap-3"
                >
                  <label
                    htmlFor="role-user"
                    className={`rounded-lg border-2 p-4 cursor-pointer transition-colors ${
                      selectedRole === ROLES.USER
                        ? 'border-blue-600 bg-blue-50'
                        : 'border-gray-200 bg-white'
                    }`}
                  >
                    <RadioGroupItem
                      id="role-user"
                      value={ROLES.USER}
                      className="sr-only"
                    />
                    <ShoppingBag className="h-5 w-5 text-gray-900" />
                    <div className="text-sm font-medium text-gray-900 mt-2">
                      Клиент
                    </div>
                    <div className="text-xs text-gray-500">
                      Заказывать доставку
                    </div>
                  </label>
                  <label
                    htmlFor="role-courier"
                    className={`rounded-lg border-2 p-4 cursor-pointer transition-colors ${
                      selectedRole === ROLES.COURIER
                        ? 'border-blue-600 bg-blue-50'
                        : 'border-gray-200 bg-white'
                    }`}
                  >
                    <RadioGroupItem
                      id="role-courier"
                      value={ROLES.COURIER}
                      className="sr-only"
                    />
                    <Bike className="h-5 w-5 text-gray-900" />
                    <div className="text-sm font-medium text-gray-900 mt-2">
                      Курьер
                    </div>
                    <div className="text-xs text-gray-500">
                      Работать курьером
                    </div>
                  </label>
                </RadioGroup>
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        <div className="grid grid-cols-2 gap-4">
          <FormField
            control={form.control}
            name="first_name"
            render={({ field }) => (
              <FormItem>
                <FormLabel>Имя</FormLabel>
                <FormControl>
                  <Input placeholder="Иван" autoComplete="given-name" {...field} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name="last_name"
            render={({ field }) => (
              <FormItem>
                <FormLabel>Фамилия</FormLabel>
                <FormControl>
                  <Input
                    placeholder="Иванов"
                    autoComplete="family-name"
                    {...field}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
        </div>

        <FormField
          control={form.control}
          name="email"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Email</FormLabel>
              <FormControl>
                <Input
                  type="email"
                  placeholder="you@example.com"
                  autoComplete="email"
                  {...field}
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        <FormField
          control={form.control}
          name="password"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Пароль</FormLabel>
              <FormControl>
                <Input
                  type="password"
                  placeholder="Не менее 8 символов"
                  autoComplete="new-password"
                  {...field}
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        <FormField
          control={form.control}
          name="phone"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Телефон</FormLabel>
              <FormControl>
                <Input
                  type="tel"
                  autoComplete="tel"
                  placeholder="+7 999 123-45-67"
                  {...field}
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        <Button
          type="submit"
          className="w-full"
          disabled={form.formState.isSubmitting}
          aria-busy={form.formState.isSubmitting}
        >
          {form.formState.isSubmitting ? (
            <>
              <Loader2 className="h-4 w-4 animate-spin mr-2" />
              Загрузка...
            </>
          ) : (
            'Зарегистрироваться'
          )}
        </Button>
      </form>
    </Form>
  );
}
