import { Link } from 'react-router-dom';
import { Package } from 'lucide-react';
import { Card } from '@/components/ui/card';
import { LoginForm } from '@/components/auth/LoginForm';

export function LoginPage() {
  return (
    <div className="bg-gray-50 min-h-screen flex items-center justify-center py-12 px-4">
      <Card className="bg-white w-full max-w-md p-8">
        <div className="mb-8">
          <Package className="h-6 w-6 text-gray-900" />
          <h1 className="text-2xl font-semibold text-gray-900 mt-4">Войти</h1>
          <p className="text-sm text-gray-500">Введите email и пароль</p>
        </div>
        <LoginForm />
        <div className="mt-6 text-center text-sm text-gray-500">
          Нет аккаунта?{' '}
          <Link to="/register" className="text-blue-600 hover:underline">
            Зарегистрироваться
          </Link>
        </div>
      </Card>
    </div>
  );
}
