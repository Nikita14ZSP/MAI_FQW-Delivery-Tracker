import axios, { AxiosError } from 'axios';
import type { AxiosRequestConfig, InternalAxiosRequestConfig } from 'axios';

const BASE_URL = import.meta.env.VITE_API_BASE_URL ?? '/v1';

export const ACCESS_TOKEN_KEY = 'access_token';
export const REFRESH_TOKEN_KEY = 'refresh_token';

export const api = axios.create({
  baseURL: BASE_URL,
  headers: { 'Content-Type': 'application/json' },
});

// Request interceptor: attach access token
api.interceptors.request.use((config: InternalAxiosRequestConfig) => {
  const token = localStorage.getItem(ACCESS_TOKEN_KEY);
  if (token && config.headers) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

// Refresh mutex: single in-flight promise shared across concurrent 401s
let refreshPromise: Promise<string> | null = null;

type RetryableRequest = AxiosRequestConfig & { _retry?: boolean };

async function performRefresh(): Promise<string> {
  const refreshToken = localStorage.getItem(REFRESH_TOKEN_KEY);
  if (!refreshToken) throw new Error('no refresh token');
  const resp = await axios.post<{ access_token: string; refresh_token: string }>(
    `${BASE_URL}/auth/refresh`,
    { refresh_token: refreshToken },
    { headers: { 'Content-Type': 'application/json' } },
  );
  localStorage.setItem(ACCESS_TOKEN_KEY, resp.data.access_token);
  localStorage.setItem(REFRESH_TOKEN_KEY, resp.data.refresh_token);
  return resp.data.access_token;
}

function onRefreshFailed() {
  localStorage.removeItem(ACCESS_TOKEN_KEY);
  localStorage.removeItem(REFRESH_TOKEN_KEY);
  // Emit a DOM event so AuthContext can react without a circular import
  window.dispatchEvent(new CustomEvent('auth:logout'));
}

api.interceptors.response.use(
  (response) => response,
  async (error: AxiosError) => {
    const original = error.config as RetryableRequest | undefined;
    if (!original || error.response?.status !== 401 || original._retry) {
      return Promise.reject(error);
    }
    // Never try to refresh on auth endpoints (login / register / refresh themselves)
    // 401 from /auth/login means bad credentials, not an expired token.
    if (
      original.url?.includes('/auth/refresh') ||
      original.url?.includes('/auth/login') ||
      original.url?.includes('/auth/register')
    ) {
      if (original.url?.includes('/auth/refresh')) {
        onRefreshFailed();
      }
      return Promise.reject(error);
    }
    original._retry = true;

    try {
      if (!refreshPromise) {
        refreshPromise = performRefresh().finally(() => {
          refreshPromise = null;
        });
      }
      const newToken = await refreshPromise;
      if (original.headers) {
        (original.headers as Record<string, string>).Authorization = `Bearer ${newToken}`;
      }
      return api(original);
    } catch (refreshErr) {
      onRefreshFailed();
      return Promise.reject(refreshErr);
    }
  },
);
