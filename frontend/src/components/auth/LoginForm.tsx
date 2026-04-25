import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { useNavigate } from 'react-router-dom';
import { Loader2 } from 'lucide-react';
import { isAxiosError } from 'axios';
import { LoginSchema, type LoginInput } from '@/lib/schemas/auth';
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
import { useToast } from '@/components/ui/use-toast';

export interface LoginFormProps {
  onSuccess?: () => void;
}

export function LoginForm({ onSuccess }: LoginFormProps) {
  const { login } = useAuth();
  const { toast } = useToast();
  const navigate = useNavigate();

  const form = useForm<LoginInput>({
    resolver: zodResolver(LoginSchema),
    defaultValues: { email: '', password: '' },
    mode: 'onSubmit',
  });

  async function onSubmit(values: LoginInput) {
    try {
      const user = await login(values);
      // Role-based redirect (D-18). user.role comes from backend response.
      const target = user.role === ROLES.COURIER ? '/courier' : '/orders';
      navigate(target, { replace: true });
      onSuccess?.();
    } catch (err) {
      if (isAxiosError(err) && err.response?.status === 401) {
        toast({
          variant: 'destructive',
          title: 'Неверный email или пароль',
        });
        // D-06: highlight fields without clearing values
        form.setError('email', { type: 'server', message: '' });
        form.setError('password', { type: 'server', message: '' });
        return;
      }
      toast({ variant: 'destructive', title: 'Ошибка сервера. Попробуйте позже' });
    }
  }

  return (
    <Form {...form}>
      <form
        onSubmit={form.handleSubmit(onSubmit)}
        className="flex flex-col gap-4"
        aria-label="login form"
        noValidate
      >
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
                  placeholder="Ваш пароль"
                  autoComplete="current-password"
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
            'Войти'
          )}
        </Button>
      </form>
    </Form>
  );
}
