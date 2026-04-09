export const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
export const WS_URL = __ENV.WS_URL || 'ws://localhost:8081';

export const THRESHOLDS = {
  http_req_duration: ['p(95)<200'],
  'http_req_duration{name:create_order}': ['p(99)<300'],
};
