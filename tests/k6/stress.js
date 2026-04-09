import http from 'k6/http';
import { check } from 'k6';
import { BASE_URL } from './lib/config.js';
import { register, login, authHeaders } from './lib/auth.js';

export const options = {
  scenarios: {
    stress: {
      executor: 'ramping-arrival-rate',
      startRate: 5,
      timeUnit: '1s',
      preAllocatedVUs: 50,
      maxVUs: 200,
      stages: [
        { duration: '1m', target: 50 },
        { duration: '2m', target: 100 },
        { duration: '1m', target: 150 },
        { duration: '30s', target: 0 },
      ],
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<500'],
    'http_req_duration{name:create_order}': ['p(99)<1000'],
  },
};

export function setup() {
  const email = `stress-user-${Date.now()}@test.com`;
  register(email, 'password123');
  const token = login(email, 'password123');
  return { token };
}

export default function (data) {
  const res = http.post(
    `${BASE_URL}/v1/orders`,
    JSON.stringify({
      deliveryAddress: `Stress ${__ITER}`,
      deliveryCoordinates: { latitude: 55.7558, longitude: 37.6173 },
      items: [{ name: 'Item', quantity: 1, price: 500 }],
      contactPhone: '+79001234567',
      paymentMethod: 'cash',
    }),
    { ...authHeaders(data.token), tags: { name: 'create_order' } },
  );
  check(res, { 'status ok': (r) => r.status === 200 || r.status === 429 });
}
