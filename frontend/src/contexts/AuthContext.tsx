import { createContext, useCallback, useEffect, useMemo, useState, type ReactNode } from 'react';
import { jwtDecode } from 'jwt-decode';
import { api, ACCESS_TOKEN_KEY, REFRESH_TOKEN_KEY } from '@/lib/api';
import type { AuthResponse, JwtPayload, User } from '@/types/auth';
import type { RegisterInput, LoginInput } from '@/lib/schemas/auth';

export interface AuthContextValue {
  user: User | null;
  loading: boolean;
  login: (input: LoginInput) => Promise<User>;
  register: (input: RegisterInput) => Promise<User>;
  logout: () => void;
}

export const AuthContext = createContext<AuthContextValue | null>(null);

function hydrateFromStorage(): User | null {
  const token = localStorage.getItem(ACCESS_TOKEN_KEY);
  if (!token) return null;
  try {
    const payload = jwtDecode<JwtPayload>(token);
    if (payload.exp * 1000 <= Date.now()) return null;
    // Minimal user from JWT claims — first_name/last_name unknown at bootstrap
    return {
      id: payload.user_id,
      email: '',
      first_name: '',
      last_name: '',
      role: payload.role,
    };
  } catch {
    return null;
  }
}

function storeTokensAndUser(
  tokens: { access_token: string; refresh_token: string },
  user: User,
): User {
  localStorage.setItem(ACCESS_TOKEN_KEY, tokens.access_token);
  localStorage.setItem(REFRESH_TOKEN_KEY, tokens.refresh_token);
  return user;
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(() => hydrateFromStorage());
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    const handleLogout = () => setUser(null);
    window.addEventListener('auth:logout', handleLogout);
    return () => window.removeEventListener('auth:logout', handleLogout);
  }, []);

  const login = useCallback(async (input: LoginInput) => {
    setLoading(true);
    try {
      const { data } = await api.post<AuthResponse>('/auth/login', input);
      const u = storeTokensAndUser(
        { access_token: data.access_token, refresh_token: data.refresh_token },
        data.user,
      );
      setUser(u);
      return u;
    } finally {
      setLoading(false);
    }
  }, []);

  const register = useCallback(async (input: RegisterInput) => {
    setLoading(true);
    try {
      const { data } = await api.post<AuthResponse>('/auth/register', input);
      const u = storeTokensAndUser(
        { access_token: data.access_token, refresh_token: data.refresh_token },
        data.user,
      );
      setUser(u);
      return u;
    } finally {
      setLoading(false);
    }
  }, []);

  const logout = useCallback(() => {
    localStorage.removeItem(ACCESS_TOKEN_KEY);
    localStorage.removeItem(REFRESH_TOKEN_KEY);
    setUser(null);
  }, []);

  const value = useMemo<AuthContextValue>(
    () => ({ user, loading, login, register, logout }),
    [user, loading, login, register, logout],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}
