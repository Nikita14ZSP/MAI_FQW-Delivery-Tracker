import { http, HttpResponse } from 'msw';

const BASE = '/v1';

const sampleOrderRaw = (id = 'ord-1', status = 'ORDER_STATUS_CREATED') => ({
  id,
  userId: 'user-1',
  status,
  delivery_id: 'dlv-mock-1',
  deliveryAddress: 'Москва, ул. Тверская, 1',
  deliveryCoordinates: { latitude: 55.7558, longitude: 37.6173 },
  items: [{ name: 'Пицца Маргарита', quantity: 1, price: 520 }],
  contactPhone: '+79991234567',
  paymentMethod: 'card_on_delivery',
  createdAt: '2026-05-09T10:00:00Z',
  updatedAt: '2026-05-09T10:00:00Z',
});

export const authHandlers = [
  http.post(`${BASE}/auth/register`, async ({ request }) => {
    const body = (await request.json()) as { email: string; role?: string };
    return HttpResponse.json(
      {
        user: {
          id: 'u-1',
          email: body.email,
          first_name: 'Test',
          last_name: 'User',
          role: body.role ?? 'user',
        },
        access_token: 'mock-access-token',
        refresh_token: 'mock-refresh-token',
      },
      { status: 201 },
    );
  }),
  http.post(`${BASE}/auth/login`, async ({ request }) => {
    const body = (await request.json()) as { email: string };
    return HttpResponse.json({
      user: {
        id: 'u-1',
        email: body.email,
        first_name: 'Test',
        last_name: 'User',
        role: 'user',
      },
      access_token: 'mock-access-token',
      refresh_token: 'mock-refresh-token',
    });
  }),
  http.post(`${BASE}/auth/refresh`, async () => {
    return HttpResponse.json({
      access_token: 'new-access-token',
      refresh_token: 'new-refresh-token',
    });
  }),
];

export const orderHandlers = [
  http.get(`${BASE}/orders`, () =>
    HttpResponse.json({
      orders: [sampleOrderRaw()],
      pagination: { total: 1, page: 1, pageSize: 100 },
    }),
  ),
  http.get(`${BASE}/orders/:id`, ({ params }) =>
    HttpResponse.json({ order: sampleOrderRaw(params.id as string) }),
  ),
  http.post(`${BASE}/orders`, async () => {
    return HttpResponse.json({ order: sampleOrderRaw('ord-new') }, { status: 201 });
  }),
  http.post(`${BASE}/orders/:id/cancel`, ({ params }) =>
    HttpResponse.json({
      order: sampleOrderRaw(params.id as string, 'ORDER_STATUS_CANCELLED'),
    }),
  ),
];

export const menuHandlers = [
  http.get(`${BASE}/menu/categories`, () =>
    HttpResponse.json({
      categories: [
        { id: 'cat-1', name: 'Закуски', sortOrder: 1 },
        { id: 'cat-2', name: 'Горячее', sortOrder: 2 },
      ],
    }),
  ),
  http.get(`${BASE}/menu/items`, ({ request }) => {
    const url = new URL(request.url);
    const items = [
      {
        id: 'item-1',
        categoryId: 'cat-1',
        name: 'Цезарь',
        price: 390,
        imageUrl: 'https://example.com/c.jpg',
        available: true,
      },
      {
        id: 'item-2',
        categoryId: 'cat-2',
        name: 'Пицца',
        price: 520,
        imageUrl: 'https://example.com/p.jpg',
        available: true,
      },
    ];
    const cat = url.searchParams.get('category_id');
    return HttpResponse.json({
      items: cat ? items.filter((i) => i.categoryId === cat) : items,
    });
  }),
];

// --- Phase 9: Tracking endpoints (per 09-RESEARCH §Validation Architecture + Pitfall 7) ---
export const trackingHandlers = [
  // GET /v1/deliveries/by-order/:order_id → seed delivery.estimated_delivery for ETA + courier_id.
  // Returns the REAL backend camelCase enveloped shape (grpc-gateway default) so tests exercise
  // the normalizeDelivery boundary mapper — see quick-260516-72y.
  http.get(`${BASE}/deliveries/by-order/:orderId`, ({ params }) =>
    HttpResponse.json({
      delivery: {
        id: 'dlv-mock-1',
        orderId: params.orderId,
        courierId: 'crr-mock-1',
        zoneId: 'zone-1',
        status: 'DELIVERY_STATUS_ASSIGNED',
        estimatedDelivery: '2026-05-16T02:11:18.531098Z',
        createdAt: '2026-05-16T01:41:13.994964Z',
      },
    }),
  ),
  // GET /v1/tracking/history/:courier_id?order_id=...&limit=N
  // Per Pitfall 7: keyed by courier_id, order_id passed as query.
  // Emits the REAL live-confirmed grpc-gateway nested camelCase shape so tests
  // exercise the normalizeLocationPoint boundary mapper — see quick-260516-e30
  // (mirrors the normalizeDelivery by-order mock from quick-260516-72y).
  http.get(`${BASE}/tracking/history/:courierId`, ({ params }) =>
    HttpResponse.json({
      locations: [
        {
          courierId: params.courierId,
          orderId: 'order-mock-1',
          coordinates: { latitude: 55.7558, longitude: 37.6173 },
          timestamp: '2026-05-15T12:05:00Z',
        },
        {
          courierId: params.courierId,
          orderId: 'order-mock-1',
          coordinates: { latitude: 55.756, longitude: 37.618 },
          timestamp: '2026-05-15T12:06:00Z',
        },
      ],
    }),
  ),
  // GET /v1/tracking/couriers/:courier_id/location → last cached position from Redis.
  // Live-confirmed envelope: { location: { courierId, orderId,
  // coordinates:{latitude,longitude}, timestamp } } (quick-260516-e30 probe).
  http.get(`${BASE}/tracking/couriers/:courierId/location`, ({ params }) =>
    HttpResponse.json({
      location: {
        courierId: params.courierId,
        orderId: 'order-mock-1',
        coordinates: { latitude: 55.7565, longitude: 37.619 },
        timestamp: '2026-05-15T12:10:00Z',
      },
    }),
  ),
];

// --- Phase 10: Courier dashboard endpoints (per 10-RESEARCH.md §"Live-Confirmed API Response Shapes") ---
// IMPORTANT: All responses use the REAL grpc-gateway camelCase shape (Pitfall 5).
// Tests assert the normalized snake_case form — never use snake_case keys in these handlers.
export const courierHandlers = [
  // GET /v1/couriers/:courierId/rating
  // Live shape: { averageStars, totalRatings, ratings: [{deliveryId,stars,comment,createdAt}] }
  http.get(`${BASE}/couriers/:courierId/rating`, () =>
    HttpResponse.json({
      averageStars: 4.5,
      totalRatings: 2,
      ratings: [
        {
          deliveryId: 'dlv-1',
          stars: 4,
          comment: 'Быстро',
          createdAt: '2026-05-16T08:00:00Z',
        },
      ],
    }),
  ),

  // GET /v1/couriers/:courierId/profile (public — RATE-05)
  // Live shape: { firstName, lastName, averageStars, totalRatings } (camelCase, Pitfall 5)
  http.get(`${BASE}/couriers/:courierId/profile`, () =>
    HttpResponse.json({
      firstName: 'Иван',
      lastName: 'Иванов',
      averageStars: 4.5,
      totalRatings: 12,
    }),
  ),

  // PATCH /v1/couriers/me/status
  // Reads body {status} and echoes it back in the { courier: {...} } envelope.
  // Live shape: { courier: { id, status, updatedAt } }
  http.patch(`${BASE}/couriers/me/status`, async ({ request }) => {
    const body = (await request.json()) as { status: string };
    return HttpResponse.json({
      courier: {
        id: 'crr-mock-1',
        status: body.status,
        updatedAt: '2026-05-16T08:05:53.898844Z',
      },
    });
  }),

  // GET /v1/couriers/me/available-orders
  // Live shape: { orders: [AvailableOrderPreviewRaw], nextCursor }
  // Non-empty array per Pitfall 2 (live env auto-assigns immediately)
  http.get(`${BASE}/couriers/me/available-orders`, () =>
    HttpResponse.json({
      orders: [
        {
          orderId: 'ord-av-1',
          deliveryId: 'dlv-av-1',
          deliveryAddress: 'Москва, Тверская, 1',
          totalPrice: 520,
          itemsCount: 2,
          zoneName: 'Test Zone',
          createdAt: '2026-05-16T07:00:00Z',
          items: [
            { name: 'Пицца Маргарита', quantity: 1, price: 390 },
            { name: 'Напиток', quantity: 1, price: 130 },
          ],
        },
      ],
      nextCursor: '',
    }),
  ),

  // GET /v1/couriers/:courierId/deliveries
  // Live shape: { deliveries: [DeliveryRaw] } — matches existing DeliveryRaw in types/tracking.ts
  http.get(`${BASE}/couriers/:courierId/deliveries`, ({ params }) =>
    HttpResponse.json({
      deliveries: [
        {
          id: 'dlv-act-1',
          orderId: 'ord-act-1',
          courierId: params.courierId,
          zoneId: 'zone-1',
          status: 'DELIVERY_STATUS_ASSIGNED',
          estimatedDelivery: '2026-05-16T08:30:00Z',
          createdAt: '2026-05-16T07:00:00Z',
        },
      ],
    }),
  ),

  // POST /v1/couriers/me/deliveries/:deliveryId/accept
  // Live shape: { delivery: DeliveryRaw }
  http.post(`${BASE}/couriers/me/deliveries/:deliveryId/accept`, ({ params }) =>
    HttpResponse.json({
      delivery: {
        id: params.deliveryId,
        orderId: 'ord-av-1',
        courierId: 'crr-mock-1',
        zoneId: 'zone-1',
        status: 'DELIVERY_STATUS_ASSIGNED',
        createdAt: '2026-05-16T07:00:00Z',
      },
    }),
  ),

  // PATCH /v1/orders/:id/status
  // Reads body {status} and echoes it back in the { order: OrderRaw } envelope.
  // Live shape: { order: OrderRaw } — note delivery_id stays snake_case (Pitfall 4, live quirk)
  http.patch(`${BASE}/orders/:id/status`, async ({ request, params }) => {
    const body = (await request.json()) as { status: string };
    return HttpResponse.json({
      order: {
        id: params.id,
        userId: 'user-1',
        status: body.status,
        deliveryAddress: 'Москва, Тверская, 1',
        deliveryCoordinates: { latitude: 55.7558, longitude: 37.6173 },
        items: [{ name: 'Пицца', quantity: 1, price: 520 }],
        contactPhone: '+79991234567',
        paymentMethod: 'card_on_delivery',
        createdAt: '2026-05-16T07:00:00Z',
        updatedAt: '2026-05-16T08:00:00Z',
        delivery_id: 'dlv-act-1', // snake_case quirk — live-confirmed Pitfall 4
      },
    });
  }),

  // POST /v1/tracking/location
  // Live shape: { success: true }
  http.post(`${BASE}/tracking/location`, () => HttpResponse.json({ success: true })),
];

// --- Phase 11 RATE-04: POST /v1/ratings — camelCase response per Pitfall 5 (zero snake_case) ---
export const ratingHandlers = [
  http.post(`${BASE}/ratings`, () => HttpResponse.json({ ratingId: 'rat-mock-1' })),
];

export const nominatimHandlers = [
  http.get('https://nominatim.openstreetmap.org/search', () =>
    HttpResponse.json([
      {
        place_id: 1,
        display_name: 'Москва, Тверская, 1',
        lat: '55.7558',
        lon: '37.6173',
      },
    ]),
  ),
  http.get('https://nominatim.openstreetmap.org/reverse', () =>
    HttpResponse.json({ display_name: 'Москва, Тверская, 1' }),
  ),
];

export const handlers = [
  ...authHandlers,
  ...orderHandlers,
  ...menuHandlers,
  ...trackingHandlers,
  ...nominatimHandlers,
  ...courierHandlers,
  ...ratingHandlers,
];
