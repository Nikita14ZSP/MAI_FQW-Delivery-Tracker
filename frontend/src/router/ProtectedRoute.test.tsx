import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { AuthContext, type AuthContextValue } from '@/contexts/AuthContext';
import { ProtectedRoute } from './ProtectedRoute';
import type { User } from '@/types/auth';

function buildAuth(user: User | null): AuthContextValue {
  return {
    user,
    loading: false,
    login: async () => user as User,
    register: async () => user as User,
    logout: () => {},
  };
}

function renderWithAuth(
  auth: AuthContextValue,
  initialPath: string,
  allowedRoles: Array<'user' | 'courier' | 'admin'>,
) {
  return render(
    <AuthContext.Provider value={auth}>
      <MemoryRouter initialEntries={[initialPath]}>
        <Routes>
          <Route path="/login" element={<div>LOGIN_STUB</div>} />
          <Route path="/403" element={<div>FORBIDDEN_STUB</div>} />
          <Route element={<ProtectedRoute allowedRoles={allowedRoles} />}>
            <Route path="/orders" element={<div>PROTECTED_STUB</div>} />
          </Route>
        </Routes>
      </MemoryRouter>
    </AuthContext.Provider>,
  );
}

describe('ProtectedRoute', () => {
  it('redirects to /login when user is null', () => {
    renderWithAuth(buildAuth(null), '/orders', ['user']);
    expect(screen.getByText('LOGIN_STUB')).toBeInTheDocument();
  });

  it('redirects to /403 when user role is not in allowedRoles', () => {
    const user: User = { id: 'u-1', email: 'a@b.io', first_name: 'A', last_name: 'B', role: 'user' };
    renderWithAuth(buildAuth(user), '/orders', ['courier']);
    expect(screen.getByText('FORBIDDEN_STUB')).toBeInTheDocument();
  });

  it('renders protected child when user role is in allowedRoles', () => {
    const user: User = { id: 'u-1', email: 'a@b.io', first_name: 'A', last_name: 'B', role: 'user' };
    renderWithAuth(buildAuth(user), '/orders', ['user']);
    expect(screen.getByText('PROTECTED_STUB')).toBeInTheDocument();
  });
});
