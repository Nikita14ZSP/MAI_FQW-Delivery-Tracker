import { useNavigate } from 'react-router-dom';
import { Button } from '@/components/ui/button';
import { useAuth } from '@/hooks/useAuth';

export function CourierPlaceholderPage() {
  const { logout } = useAuth();
  const navigate = useNavigate();

  function handleLogout() {
    logout();
    navigate('/login', { replace: true });
  }

  return (
    <div className="bg-gray-50 min-h-screen p-8">
      <div className="max-w-3xl mx-auto">
        <div className="flex items-center justify-between mb-6">
          <h1 className="text-2xl font-semibold text-gray-900">Курьер</h1>
          <Button variant="outline" onClick={handleLogout}>Выйти</Button>
        </div>
        <p className="text-gray-500">Скоро здесь будут ваши доставки.</p>
      </div>
    </div>
  );
}
