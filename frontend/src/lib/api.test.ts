import { describe, it, expect, beforeEach, vi } from 'vitest';
import { http, HttpResponse } from 'msw';
import { server } from '@/test/mocks/server';
import { api, ACCESS_TOKEN_KEY, REFRESH_TOKEN_KEY } from './api';

describe('api client', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it('attaches Authorization header when token present', async () => {
    localStorage.setItem(ACCESS_TOKEN_KEY, 'abc');
    let captured: string | null = null;
    server.use(
      http.get('/v1/ping', ({ request }) => {
        captured = request.headers.get('Authorization');
        return HttpResponse.json({ ok: true });
      }),
    );
    await api.get('/ping');
    expect(captured).toBe('Bearer abc');
  });

  it('does not attach Authorization header when no token', async () => {
    let captured: string | null = 'present';
    server.use(
      http.get('/v1/ping', ({ request }) => {
        captured = request.headers.get('Authorization');
        return HttpResponse.json({ ok: true });
      }),
    );
    await api.get('/ping');
    expect(captured).toBeNull();
  });

  it('refreshes token on 401 and retries original request', async () => {
    localStorage.setItem(ACCESS_TOKEN_KEY, 'expired');
    localStorage.setItem(REFRESH_TOKEN_KEY, 'rt-1');
    let calls = 0;
    server.use(
      http.get('/v1/protected', ({ request }) => {
        calls++;
        const auth = request.headers.get('Authorization');
        if (auth === 'Bearer expired') return new HttpResponse(null, { status: 401 });
        if (auth === 'Bearer new-access-token') return HttpResponse.json({ ok: true });
        return new HttpResponse(null, { status: 401 });
      }),
    );
    const resp = await api.get('/protected');
    expect(resp.status).toBe(200);
    expect(calls).toBe(2);
    expect(localStorage.getItem(ACCESS_TOKEN_KEY)).toBe('new-access-token');
  });

  it('MUTEX: parallel 401s trigger refresh exactly once', async () => {
    localStorage.setItem(ACCESS_TOKEN_KEY, 'expired');
    localStorage.setItem(REFRESH_TOKEN_KEY, 'rt-1');
    let refreshCalls = 0;
    server.use(
      http.post('/v1/auth/refresh', async () => {
        refreshCalls++;
        await new Promise((r) => setTimeout(r, 20));
        return HttpResponse.json({ access_token: 'new-access-token', refresh_token: 'new-rt' });
      }),
      http.get('/v1/protected', ({ request }) => {
        const auth = request.headers.get('Authorization');
        if (auth === 'Bearer new-access-token') return HttpResponse.json({ ok: true });
        return new HttpResponse(null, { status: 401 });
      }),
    );
    const results = await Promise.all([api.get('/protected'), api.get('/protected'), api.get('/protected')]);
    expect(refreshCalls).toBe(1);
    expect(results.every((r) => r.status === 200)).toBe(true);
  });

  it('failed refresh clears localStorage and dispatches auth:logout event', async () => {
    localStorage.setItem(ACCESS_TOKEN_KEY, 'expired');
    localStorage.setItem(REFRESH_TOKEN_KEY, 'rt-1');
    const listener = vi.fn();
    window.addEventListener('auth:logout', listener);
    server.use(
      http.post('/v1/auth/refresh', () => new HttpResponse(null, { status: 401 })),
      http.get('/v1/protected', () => new HttpResponse(null, { status: 401 })),
    );
    await expect(api.get('/protected')).rejects.toBeDefined();
    expect(localStorage.getItem(ACCESS_TOKEN_KEY)).toBeNull();
    expect(localStorage.getItem(REFRESH_TOKEN_KEY)).toBeNull();
    expect(listener).toHaveBeenCalled();
    window.removeEventListener('auth:logout', listener);
  });
});
