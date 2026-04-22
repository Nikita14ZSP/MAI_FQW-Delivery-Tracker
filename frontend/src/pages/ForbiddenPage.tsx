import { Link } from 'react-router-dom';
import { ShieldOff } from 'lucide-react';
import { Button } from '@/components/ui/button';

export function ForbiddenPage() {
  return (
    <div className="bg-gray-50 min-h-screen flex items-center justify-center">
      <div className="text-center max-w-sm px-4">
        <ShieldOff className="h-12 w-12 text-gray-400 mx-auto" />
        <h1 className="text-2xl font-semibold text-gray-900 mt-4">Доступ запрещён</h1>
        <p className="text-gray-500 mt-2">У вас нет прав для просмотра этой страницы.</p>
        <Button asChild variant="outline" className="mt-6">
          <Link to="/">Вернуться на главную</Link>
        </Button>
      </div>
    </div>
  );
}
