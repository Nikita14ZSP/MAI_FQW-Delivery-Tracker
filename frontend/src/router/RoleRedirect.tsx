import { Navigate } from 'react-router-dom';
import { useAuth } from '@/hooks/useAuth';
import { ROLES } from '@/lib/constants';

export function RoleRedirect() {
  const { user, loading } = useAuth();

  if (loading) return null;

  if (!user) {
    return <Navigate to="/login" replace />;
  }

  if (user.role === ROLES.USER) {
    return <Navigate to="/orders" replace />;
  }

  if (user.role === ROLES.COURIER) {
    return <Navigate to="/courier" replace />;
  }

  // Unknown role (e.g. 'admin' without a dashboard yet in Phase 7)
  return <Navigate to="/403" replace />;
}
