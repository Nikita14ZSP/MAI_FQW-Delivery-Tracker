import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { server } from '@/test/mocks/server';
import { AuthProvider } from '@/contexts/AuthContext';
import { RegisterForm } from './RegisterForm';
import { Toaster } from '@/components/ui/toaster';
import { ACCESS_TOKEN_KEY } from '@/lib/api';

function renderForm(onSuccess?: () => void) {
  return render(
    <AuthProvider>
      <RegisterForm onSuccess={onSuccess} />
      <Toaster />
    </AuthProvider>,
  );
}

describe('RegisterForm', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it('shows inline errors on empty submit', async () => {
    const user = userEvent.setup();
    renderForm();
    // Clear pre-filled name fields if any — defaults should be empty
    await user.click(screen.getByRole('button', { name: /Зарегистрироваться/ }));
    await waitFor(() =>
      expect(screen.getByText('Введите email')).toBeInTheDocument(),
    );
    expect(screen.getByText('Введите пароль')).toBeInTheDocument();
    expect(screen.getByText('Введите имя')).toBeInTheDocument();
    expect(screen.getByText('Введите фамилию')).toBeInTheDocument();
  });

  it('rejects short password with zod error', async () => {
    const user = userEvent.setup();
    renderForm();
    await user.type(screen.getByLabelText('Email'), 'a@b.io');
    await user.type(screen.getByLabelText('Пароль'), 'short');
    await user.type(screen.getByLabelText('Имя'), 'A');
    await user.type(screen.getByLabelText('Фамилия'), 'B');
    await user.click(screen.getByRole('button', { name: /Зарегистрироваться/ }));
    await waitFor(() =>
      expect(
        screen.getByText('Пароль должен содержать не менее 8 символов'),
      ).toBeInTheDocument(),
    );
  });

  it('submits valid form and stores tokens via AuthContext', async () => {
    const user = userEvent.setup();
    const onSuccess = vi.fn();
    renderForm(onSuccess);
    await user.type(screen.getByLabelText('Email'), 'a@b.io');
    await user.type(screen.getByLabelText('Пароль'), 'password123');
    await user.type(screen.getByLabelText('Имя'), 'Ivan');
    await user.type(screen.getByLabelText('Фамилия'), 'Ivanov');
    await user.click(screen.getByRole('button', { name: /Зарегистрироваться/ }));
    await waitFor(() => expect(onSuccess).toHaveBeenCalled());
    expect(localStorage.getItem(ACCESS_TOKEN_KEY)).toBe('mock-access-token');
  });

  it('sends role=courier when courier card selected', async () => {
    const user = userEvent.setup();
    let capturedRole: string | undefined;
    server.use(
      http.post('/v1/auth/register', async ({ request }) => {
        const body = (await request.json()) as { role: string; email: string };
        capturedRole = body.role;
        return HttpResponse.json(
          {
            user: {
              id: 'u-1',
              email: body.email,
              first_name: 'Ivan',
              last_name: 'Ivanov',
              role: body.role,
            },
            access_token: 'mock-access-token',
            refresh_token: 'mock-refresh-token',
          },
          { status: 201 },
        );
      }),
    );
    const onSuccess = vi.fn();
    renderForm(onSuccess);
    // Click the courier card
    await user.click(screen.getByText('Курьер'));
    await user.type(screen.getByLabelText('Email'), 'c@b.io');
    await user.type(screen.getByLabelText('Пароль'), 'password123');
    await user.type(screen.getByLabelText('Имя'), 'Ivan');
    await user.type(screen.getByLabelText('Фамилия'), 'Ivanov');
    await user.click(screen.getByRole('button', { name: /Зарегистрироваться/ }));
    await waitFor(() => expect(onSuccess).toHaveBeenCalled());
    expect(capturedRole).toBe('courier');
  });

  it('renders phone field with placeholder +7 999 123-45-67', () => {
    renderForm();
    expect(
      screen.getByPlaceholderText('+7 999 123-45-67'),
    ).toBeInTheDocument();
  });

  it('submits valid phone value to register endpoint', async () => {
    const user = userEvent.setup();
    let capturedPhone: string | undefined;
    server.use(
      http.post('/v1/auth/register', async ({ request }) => {
        const body = (await request.json()) as { phone?: string; email: string };
        capturedPhone = body.phone;
        return HttpResponse.json(
          {
            user: {
              id: 'u-phone',
              email: body.email,
              first_name: 'Ivan',
              last_name: 'Ivanov',
              phone: body.phone,
              role: 'user',
            },
            access_token: 'mock-access-token',
            refresh_token: 'mock-refresh-token',
          },
          { status: 201 },
        );
      }),
    );
    const onSuccess = vi.fn();
    renderForm(onSuccess);
    await user.type(screen.getByLabelText('Email'), 'phone@b.io');
    await user.type(screen.getByLabelText('Пароль'), 'password123');
    await user.type(screen.getByLabelText('Имя'), 'Ivan');
    await user.type(screen.getByLabelText('Фамилия'), 'Ivanov');
    await user.type(
      screen.getByPlaceholderText('+7 999 123-45-67'),
      '+79991234567',
    );
    await user.click(screen.getByRole('button', { name: /Зарегистрироваться/ }));
    await waitFor(() => expect(onSuccess).toHaveBeenCalled());
    expect(capturedPhone).toBe('+79991234567');
  });

  it('allows submitting empty phone (still 201)', async () => {
    const user = userEvent.setup();
    const onSuccess = vi.fn();
    renderForm(onSuccess);
    await user.type(screen.getByLabelText('Email'), 'nophone@b.io');
    await user.type(screen.getByLabelText('Пароль'), 'password123');
    await user.type(screen.getByLabelText('Имя'), 'Ivan');
    await user.type(screen.getByLabelText('Фамилия'), 'Ivanov');
    // phone left empty intentionally
    await user.click(screen.getByRole('button', { name: /Зарегистрироваться/ }));
    await waitFor(() => expect(onSuccess).toHaveBeenCalled());
  });

  it('shows inline error on invalid phone format', async () => {
    const user = userEvent.setup();
    renderForm();
    await user.type(screen.getByLabelText('Email'), 'a@b.io');
    await user.type(screen.getByLabelText('Пароль'), 'password123');
    await user.type(screen.getByLabelText('Имя'), 'Ivan');
    await user.type(screen.getByLabelText('Фамилия'), 'Ivanov');
    await user.type(screen.getByPlaceholderText('+7 999 123-45-67'), '12345');
    await user.click(screen.getByRole('button', { name: /Зарегистрироваться/ }));
    await waitFor(() =>
      expect(
        screen.getByText('Введите телефон в формате +7XXXXXXXXXX'),
      ).toBeInTheDocument(),
    );
  });

  it('handles 409 with toast + email field error', async () => {
    server.use(
      http.post('/v1/auth/register', () =>
        HttpResponse.json({ error: 'email exists' }, { status: 409 }),
      ),
    );
    const user = userEvent.setup();
    renderForm();
    await user.type(screen.getByLabelText('Email'), 'dup@b.io');
    await user.type(screen.getByLabelText('Пароль'), 'password123');
    await user.type(screen.getByLabelText('Имя'), 'Ivan');
    await user.type(screen.getByLabelText('Фамилия'), 'Ivanov');
    await user.click(screen.getByRole('button', { name: /Зарегистрироваться/ }));
    await waitFor(() =>
      expect(
        screen.getByText('Пользователь с таким email уже зарегистрирован'),
      ).toBeInTheDocument(),
    );
    expect(localStorage.getItem(ACCESS_TOKEN_KEY)).toBeNull();
  });
});
