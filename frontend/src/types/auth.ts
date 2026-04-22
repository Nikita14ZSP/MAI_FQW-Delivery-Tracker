import type { Role } from '@/lib/constants';

export interface User {
  id: string;
  email: string;
  first_name: string;
  last_name: string;
  phone?: string;
  role: Role;
}

export interface AuthTokens {
  access_token: string;
  refresh_token: string;
}

export interface AuthResponse {
  user: User;
  access_token: string;
  refresh_token: string;
}

export interface JwtPayload {
  user_id: string;
  role: Role;
  exp: number;
  iat?: number;
}
