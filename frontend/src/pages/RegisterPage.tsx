import { Link, useNavigate } from 'react-router-dom';
import { Package } from 'lucide-react';
import { Card } from '@/components/ui/card';
import { RegisterForm } from '@/components/auth/RegisterForm';
import { useAuth } from '@/hooks/useAuth';
import { ROLES } from '@/lib/constants';

export function RegisterPage() {
  const navigate = useNavigate();
  const { user } = useAuth();

  function handleSuccess() {
    // AuthContext updates user state synchronously in register().
    // Read latest role — Context state is available here after the handler runs.
    const role = user?.role;
    if (role === ROLES.COURIER) {
      navigate('/courier', { replace: true });
    } else {
      navigate('/orders', { replace: true });
    }
  }

  return (
    <div className="bg-gray-50 min-h-screen flex items-center justify-center py-12 px-4">
      <Card className="bg-white w-full max-w-md p-8">
        <div className="mb-8">
          <Package className="h-6 w-6 text-gray-900" />
          <h1 className="text-2xl font-semibold text-gray-900 mt-4">
            Создать аккаунт
          </h1>
          <p className="text-sm text-gray-500">
            Заполните данные для регистрации
          </p>
        </div>
        <RegisterForm onSuccess={handleSuccess} />
        <div className="mt-6 text-center text-sm text-gray-500">
          Уже есть аккаунт?{' '}
          <Link to="/login" className="text-blue-600 hover:underline">
            Войти
          </Link>
        </div>
      </Card>
    </div>
  );
}
