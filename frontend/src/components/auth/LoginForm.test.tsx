import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { http, HttpResponse } from 'msw';
import { server } from '@/test/mocks/server';
import { AuthProvider } from '@/contexts/AuthContext';
import { LoginForm } from './LoginForm';
import { Toaster } from '@/components/ui/toaster';
import { ACCESS_TOKEN_KEY, REFRESH_TOKEN_KEY } from '@/lib/api';

// Mock useNavigate so we can assert navigation targets
const navigateMock = vi.fn();
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom');
  return { ...actual, useNavigate: () => navigateMock };
});

function renderForm() {
  return render(
    <MemoryRouter>
      <AuthProvider>
        <LoginForm />
        <Toaster />
      </AuthProvider>
    </MemoryRouter>,
  );
}

describe('LoginForm', () => {
  beforeEach(() => {
    localStorage.clear();
    navigateMock.mockClear();
  });

  it('shows inline errors on empty submit', async () => {
    const user = userEvent.setup();
    renderForm();
    await user.click(screen.getByRole('button', { name: /Войти/ }));
    await waitFor(() => expect(screen.getByText('Введите email')).toBeInTheDocument());
    expect(screen.getByText('Введите пароль')).toBeInTheDocument();
  });

  it('shows "Некорректный email" for invalid email format', async () => {
    const user = userEvent.setup();
    renderForm();
    await user.type(screen.getByLabelText('Email'), 'nope');
    await user.type(screen.getByLabelText('Пароль'), 'password123');
    await user.click(screen.getByRole('button', { name: /Войти/ }));
    await waitFor(() => expect(screen.getByText('Некорректный email')).toBeInTheDocument());
  });

  it('submits valid credentials, stores tokens, navigates to /orders for user role', async () => {
    const user = userEvent.setup();
    renderForm();
    await user.type(screen.getByLabelText('Email'), 'a@b.io');
    await user.type(screen.getByLabelText('Пароль'), 'password123');
    await user.click(screen.getByRole('button', { name: /Войти/ }));
    await waitFor(() => expect(navigateMock).toHaveBeenCalledWith('/orders', { replace: true }));
    expect(localStorage.getItem(ACCESS_TOKEN_KEY)).toBe('mock-access-token');
    expect(localStorage.getItem(REFRESH_TOKEN_KEY)).toBe('mock-refresh-token');
  });

  it('navigates to /courier when backend returns role=courier', async () => {
    server.use(
      http.post('/v1/auth/login', async ({ request }) => {
        const body = (await request.json()) as { email: string };
        return HttpResponse.json({
          user: { id: 'u-2', email: body.email, first_name: 'C', last_name: 'X', role: 'courier' },
          access_token: 'mock-access-token',
          refresh_token: 'mock-refresh-token',
        });
      }),
    );
    const user = userEvent.setup();
    renderForm();
    await user.type(screen.getByLabelText('Email'), 'c@b.io');
    await user.type(screen.getByLabelText('Пароль'), 'password123');
    await user.click(screen.getByRole('button', { name: /Войти/ }));
    await waitFor(() => expect(navigateMock).toHaveBeenCalledWith('/courier', { replace: true }));
  });

  it('handles 401 with toast and preserves form values', async () => {
    server.use(
      http.post('/v1/auth/login', () => HttpResponse.json({ error: 'invalid creds' }, { status: 401 })),
    );
    const user = userEvent.setup();
    renderForm();
    const emailInput = screen.getByLabelText('Email') as HTMLInputElement;
    const passwordInput = screen.getByLabelText('Пароль') as HTMLInputElement;
    await user.type(emailInput, 'wrong@b.io');
    await user.type(passwordInput, 'badpass123');
    await user.click(screen.getByRole('button', { name: /Войти/ }));
    await waitFor(() =>
      expect(screen.getByText('Неверный email или пароль')).toBeInTheDocument(),
    );
    expect(localStorage.getItem(ACCESS_TOKEN_KEY)).toBeNull();
    expect(emailInput.value).toBe('wrong@b.io');
    expect(passwordInput.value).toBe('badpass123');
    expect(navigateMock).not.toHaveBeenCalled();
  });

  it('handles 500 with "Ошибка сервера" toast', async () => {
    server.use(
      http.post('/v1/auth/login', () => HttpResponse.json({ error: 'boom' }, { status: 500 })),
    );
    const user = userEvent.setup();
    renderForm();
    await user.type(screen.getByLabelText('Email'), 'a@b.io');
    await user.type(screen.getByLabelText('Пароль'), 'password123');
    await user.click(screen.getByRole('button', { name: /Войти/ }));
    await waitFor(() =>
      expect(screen.getByText('Ошибка сервера. Попробуйте позже')).toBeInTheDocument(),
    );
    expect(navigateMock).not.toHaveBeenCalled();
  });
});
