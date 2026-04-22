import { AppRoutes } from '@/router';
import { Toaster } from '@/components/ui/toaster';

export default function App() {
  return (
    <>
      <AppRoutes />
      <Toaster />
    </>
  );
}
