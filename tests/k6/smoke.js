import http from 'k6/http';
import { check, sleep } from 'k6';
import { BASE_URL, THRESHOLDS } from './lib/config.js';
import { register, login, authHeaders } from './lib/auth.js';

export const options = {
  vus: 1,
  duration: '1m',
  thresholds: THRESHOLDS,
};

export function setup() {
  const userEmail = `smoke-user-${Date.now()}@test.com`;
  register(userEmail, 'password123');
  const userToken = login(userEmail, 'password123');
  return { userToken };
}

export default function (data) {
  // 1. Create order
  const orderRes = http.post(
    `${BASE_URL}/v1/orders`,
    JSON.stringify({
      deliveryAddress: 'Москва, ул. Тестовая, ' + __ITER,
      deliveryCoordinates: { latitude: 55.7558, longitude: 37.6173 },
      items: [{ name: 'Test Item', quantity: 1, price: 500 }],
      contactPhone: '+79001234567',
      paymentMethod: 'cash',
    }),
    { ...authHeaders(data.userToken), tags: { name: 'create_order' } },
  );
  check(orderRes, {
    'create order ok': (r) => r.status === 200,
  });

  // 2. List orders
  const listRes = http.get(`${BASE_URL}/v1/orders`, authHeaders(data.userToken));
  check(listRes, { 'list orders': (r) => r.status === 200 });

  // 3. Get order by ID
  if (orderRes.status === 200) {
    const orderId = JSON.parse(orderRes.body).order.id;
    const getRes = http.get(
      `${BASE_URL}/v1/orders/${orderId}`,
      authHeaders(data.userToken),
    );
    check(getRes, { 'get order': (r) => r.status === 200 });
  }

  // 4. Get courier rating (needs auth)
  const ratingRes = http.get(
    `${BASE_URL}/v1/couriers/00000000-0000-0000-0000-000000000000/rating`,
    authHeaders(data.userToken),
  );
  check(ratingRes, { 'get rating': (r) => r.status === 200 });

  // 5. Health check
  const healthRes = http.get(`${BASE_URL}/health`);
  check(healthRes, { 'health': (r) => r.status === 200 });

  sleep(3);
}
