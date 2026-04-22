import { describe, it, expect, beforeEach } from 'vitest';
import { render, screen, act, waitFor } from '@testing-library/react';
import { AuthProvider } from './AuthContext';
import { useAuth } from '@/hooks/useAuth';
import { ACCESS_TOKEN_KEY, REFRESH_TOKEN_KEY } from '@/lib/api';

// Build a fake unsigned JWT with given payload (base64url segments, no signature validation on frontend)
function makeJwt(payload: Record<string, unknown>): string {
  const b64 = (obj: unknown) => btoa(JSON.stringify(obj)).replace(/=/g, '').replace(/\+/g, '-').replace(/\//g, '_');
  return `${b64({ alg: 'HS256', typ: 'JWT' })}.${b64(payload)}.sig`;
}

function Probe() {
  const { user, login, register, logout } = useAuth();
  return (
    <div>
      <div data-testid="user">{user ? `${user.id}:${user.role}` : 'none'}</div>
      <button onClick={() => login({ email: 'a@b.io', password: 'password123' })}>login</button>
      <button
        onClick={() =>
          register({ role: 'courier', email: 'c@b.io', password: 'password123', first_name: 'C', last_name: 'X' })
        }
      >
        register
      </button>
      <button onClick={logout}>logout</button>
    </div>
  );
}

describe('AuthContext', () => {
  beforeEach(() => localStorage.clear());

  it('bootstrap: no token → user is null', () => {
    render(
      <AuthProvider>
        <Probe />
      </AuthProvider>,
    );
    expect(screen.getByTestId('user').textContent).toBe('none');
  });

  it('bootstrap: valid non-expired JWT → user hydrated from claims', () => {
    const token = makeJwt({ user_id: 'u-42', role: 'user', exp: Math.floor(Date.now() / 1000) + 600 });
    localStorage.setItem(ACCESS_TOKEN_KEY, token);
    render(
      <AuthProvider>
        <Probe />
      </AuthProvider>,
    );
    expect(screen.getByTestId('user').textContent).toBe('u-42:user');
  });

  it('bootstrap: expired JWT → user remains null', () => {
    const token = makeJwt({ user_id: 'u-42', role: 'user', exp: Math.floor(Date.now() / 1000) - 10 });
    localStorage.setItem(ACCESS_TOKEN_KEY, token);
    render(
      <AuthProvider>
        <Probe />
      </AuthProvider>,
    );
    expect(screen.getByTestId('user').textContent).toBe('none');
  });

  it('login stores tokens and updates user', async () => {
    render(
      <AuthProvider>
        <Probe />
      </AuthProvider>,
    );
    await act(async () => {
      screen.getByText('login').click();
    });
    await waitFor(() => expect(screen.getByTestId('user').textContent).toBe('u-1:user'));
    expect(localStorage.getItem(ACCESS_TOKEN_KEY)).toBe('mock-access-token');
    expect(localStorage.getItem(REFRESH_TOKEN_KEY)).toBe('mock-refresh-token');
  });

  it('register (courier) stores tokens and updates user', async () => {
    render(
      <AuthProvider>
        <Probe />
      </AuthProvider>,
    );
    await act(async () => {
      screen.getByText('register').click();
    });
    await waitFor(() => expect(screen.getByTestId('user').textContent).toBe('u-1:courier'));
  });

  it('logout clears tokens and user', async () => {
    localStorage.setItem(
      ACCESS_TOKEN_KEY,
      makeJwt({ user_id: 'u-1', role: 'user', exp: Math.floor(Date.now() / 1000) + 600 }),
    );
    localStorage.setItem(REFRESH_TOKEN_KEY, 'rt');
    render(
      <AuthProvider>
        <Probe />
      </AuthProvider>,
    );
    expect(screen.getByTestId('user').textContent).toBe('u-1:user');
    await act(async () => {
      screen.getByText('logout').click();
    });
    expect(screen.getByTestId('user').textContent).toBe('none');
    expect(localStorage.getItem(ACCESS_TOKEN_KEY)).toBeNull();
  });

  it('auth:logout event triggers state reset', async () => {
    localStorage.setItem(
      ACCESS_TOKEN_KEY,
      makeJwt({ user_id: 'u-1', role: 'user', exp: Math.floor(Date.now() / 1000) + 600 }),
    );
    render(
      <AuthProvider>
        <Probe />
      </AuthProvider>,
    );
    expect(screen.getByTestId('user').textContent).toBe('u-1:user');
    await act(async () => {
      window.dispatchEvent(new CustomEvent('auth:logout'));
    });
    expect(screen.getByTestId('user').textContent).toBe('none');
  });
});
