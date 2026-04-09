import http from 'k6/http';
import { check } from 'k6';
import { BASE_URL } from './config.js';

const tokenCache = {};

export function register(email, password) {
  const res = http.post(
    `${BASE_URL}/v1/auth/register`,
    JSON.stringify({
      email: email,
      password: password,
      first_name: 'Load',
      last_name: 'Test',
    }),
    { headers: { 'Content-Type': 'application/json' } },
  );
  return res;
}

export function login(email, password) {
  if (tokenCache[email]) {
    return tokenCache[email];
  }
  const res = http.post(
    `${BASE_URL}/v1/auth/login`,
    JSON.stringify({
      email: email,
      password: password,
    }),
    { headers: { 'Content-Type': 'application/json' } },
  );
  check(res, { 'login status 200': (r) => r.status === 200 });
  if (res.status === 200) {
    const body = JSON.parse(res.body);
    tokenCache[email] = body.access_token;
    return body.access_token;
  }
  return null;
}

export function authHeaders(token) {
  return {
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
  };
}
