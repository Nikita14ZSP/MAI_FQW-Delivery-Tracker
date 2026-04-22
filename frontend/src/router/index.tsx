import { Routes, Route } from 'react-router-dom';
import { ProtectedRoute } from './ProtectedRoute';
import { RoleRedirect } from './RoleRedirect';
import { LoginPage } from '@/pages/LoginPage';
import { RegisterPage } from '@/pages/RegisterPage';
import { ForbiddenPage } from '@/pages/ForbiddenPage';
import { OrdersListPage } from '@/pages/OrdersListPage';
import { OrderCreatePage } from '@/pages/OrderCreatePage';
import { OrderDetailPage } from '@/pages/OrderDetailPage';
import { OrderTrackingPage } from '@/pages/OrderTrackingPage';
import { CourierDashboardPage } from '@/pages/courier/CourierDashboardPage';

export function AppRoutes() {
  return (
    <Routes>
      <Route path="/" element={<RoleRedirect />} />
      <Route path="/login" element={<LoginPage />} />
      <Route path="/register" element={<RegisterPage />} />
      <Route path="/403" element={<ForbiddenPage />} />

      {/* Client-only routes */}
      <Route element={<ProtectedRoute allowedRoles={['user']} />}>
        <Route path="/orders" element={<OrdersListPage />} />
        <Route path="/orders/new" element={<OrderCreatePage />} />
        <Route path="/orders/:id" element={<OrderDetailPage />} />
        <Route path="/orders/:id/track" element={<OrderTrackingPage />} />
      </Route>

      {/* Courier-only routes */}
      <Route element={<ProtectedRoute allowedRoles={['courier']} />}>
        <Route path="/courier" element={<CourierDashboardPage />} />
      </Route>

      {/* Catch-all — show 403 for now (404 page can be added later) */}
      <Route path="*" element={<ForbiddenPage />} />
    </Routes>
  );
}
