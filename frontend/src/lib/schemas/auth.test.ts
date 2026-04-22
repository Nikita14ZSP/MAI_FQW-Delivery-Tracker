import { describe, it, expect } from 'vitest';
import { RegisterSchema, LoginSchema } from './auth';

describe('auth schemas', () => {
  it('RegisterSchema accepts valid user input', () => {
    const result = RegisterSchema.safeParse({
      role: 'user',
      email: 'a@b.io',
      password: 'password123',
      first_name: 'A',
      last_name: 'B',
    });
    expect(result.success).toBe(true);
  });

  it('RegisterSchema rejects short password', () => {
    const result = RegisterSchema.safeParse({
      role: 'user',
      email: 'a@b.io',
      password: 'short',
      first_name: 'A',
      last_name: 'B',
    });
    expect(result.success).toBe(false);
  });

  it('LoginSchema rejects invalid email', () => {
    const result = LoginSchema.safeParse({ email: 'nope', password: 'x' });
    expect(result.success).toBe(false);
  });

  it('RegisterSchema accepts valid +7XXXXXXXXXX phone', () => {
    const result = RegisterSchema.safeParse({
      role: 'user',
      email: 'a@b.io',
      password: 'password123',
      first_name: 'A',
      last_name: 'B',
      phone: '+79991234567',
    });
    expect(result.success).toBe(true);
  });

  it('RegisterSchema rejects invalid phone format', () => {
    const result = RegisterSchema.safeParse({
      role: 'user',
      email: 'a@b.io',
      password: 'password123',
      first_name: 'A',
      last_name: 'B',
      phone: '12345',
    });
    expect(result.success).toBe(false);
    if (!result.success) {
      const phoneIssue = result.error.issues.find((i) => i.path[0] === 'phone');
      expect(phoneIssue?.message).toContain('+7XXXXXXXXXX');
    }
  });

  it('RegisterSchema allows empty phone', () => {
    const withEmpty = RegisterSchema.safeParse({
      role: 'user',
      email: 'a@b.io',
      password: 'password123',
      first_name: 'A',
      last_name: 'B',
      phone: '',
    });
    expect(withEmpty.success).toBe(true);

    const withoutKey = RegisterSchema.safeParse({
      role: 'user',
      email: 'a@b.io',
      password: 'password123',
      first_name: 'A',
      last_name: 'B',
    });
    expect(withoutKey.success).toBe(true);
  });
});
