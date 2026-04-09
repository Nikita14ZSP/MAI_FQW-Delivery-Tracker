import http from 'k6/http';
import { check } from 'k6';
import { BASE_URL, THRESHOLDS } from './lib/config.js';
import { register, login, authHeaders } from './lib/auth.js';

export const options = {
  scenarios: {
    orders: {
      executor: 'ramping-arrival-rate',
      startRate: 1,
      timeUnit: '1s',
      preAllocatedVUs: 20,
      maxVUs: 50,
      stages: [
        { duration: '1m', target: 10 },
        { duration: '2m', target: 10 },
        { duration: '30s', target: 0 },
      ],
      exec: 'orderScenario',
    },
    reads: {
      executor: 'ramping-arrival-rate',
      startRate: 1,
      timeUnit: '1s',
      preAllocatedVUs: 20,
      maxVUs: 50,
      stages: [
        { duration: '1m', target: 10 },
        { duration: '2m', target: 10 },
        { duration: '30s', target: 0 },
      ],
      exec: 'readScenario',
    },
    ratings: {
      executor: 'ramping-arrival-rate',
      startRate: 1,
      timeUnit: '1s',
      preAllocatedVUs: 10,
      maxVUs: 30,
      stages: [
        { duration: '1m', target: 5 },
        { duration: '2m', target: 5 },
        { duration: '30s', target: 0 },
      ],
      exec: 'ratingScenario',
    },
    health: {
      executor: 'ramping-arrival-rate',
      startRate: 1,
      timeUnit: '1s',
      preAllocatedVUs: 10,
      maxVUs: 30,
      stages: [
        { duration: '1m', target: 5 },
        { duration: '2m', target: 5 },
        { duration: '30s', target: 0 },
      ],
      exec: 'healthScenario',
    },
  },
  thresholds: THRESHOLDS,
};

export function setup() {
  const userEmail = `load-user-${Date.now()}@test.com`;
  register(userEmail, 'password123');
  const userToken = login(userEmail, 'password123');
  return { userToken };
}

export function orderScenario(data) {
  const createRes = http.post(
    `${BASE_URL}/v1/orders`,
    JSON.stringify({
      deliveryAddress: `Load Test ${__ITER}`,
      deliveryCoordinates: { latitude: 55.7558, longitude: 37.6173 },
      items: [{ name: 'Item', quantity: 1, price: 500 }],
      contactPhone: '+79001234567',
      paymentMethod: 'cash',
    }),
    { ...authHeaders(data.userToken), tags: { name: 'create_order' } },
  );
  check(createRes, { 'create order': (r) => r.status === 200 || r.status === 429 });
}

export function readScenario(data) {
  const listRes = http.get(`${BASE_URL}/v1/orders`, authHeaders(data.userToken));
  check(listRes, { 'list orders': (r) => r.status === 200 || r.status === 429 });
}

export function ratingScenario(data) {
  const res = http.get(
    `${BASE_URL}/v1/couriers/00000000-0000-0000-0000-000000000000/rating`,
    { ...authHeaders(data.userToken), tags: { name: 'get_rating' } },
  );
  check(res, { 'get rating': (r) => r.status === 200 || r.status === 429 });
}

export function healthScenario() {
  const res = http.get(`${BASE_URL}/health`);
  check(res, { 'health ok': (r) => r.status === 200 || r.status === 429 });
}
