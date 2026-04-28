import { describe, it, expect, vi, beforeEach } from 'vitest';
import { createElement, type ReactNode } from 'react';
import { screen, waitFor, act } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { renderWithProviders } from '@/test/test-utils';
import { server } from '@/test/mocks/server';
import type { User } from '@/types/auth';

// Mock react-leaflet primitives BEFORE importing OrderCreatePage (jsdom can't compute Leaflet layout).
vi.mock('react-leaflet', () => ({
  MapContainer: ({
    children,
    role,
    'aria-label': ariaLabel,
  }: {
    children?: ReactNode;
    role?: string;
    'aria-label'?: string;
    [key: string]: unknown;
  }) =>
    createElement(
      'div',
      { 'data-testid': 'map-container', role, 'aria-label': ariaLabel },
      children,
    ),
  TileLayer: () => createElement('div', { 'data-testid': 'tile-layer' }),
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  Marker: ({ position }: any) =>
    createElement('div', {
      'data-testid': 'pin-marker',
      'data-lat': position[0],
      'data-lon': position[1],
    }),
  useMap: () => ({ flyTo: vi.fn() }),
  useMapEvents: () => null,
}));

vi.mock('@/lib/leaflet-icon-fix', () => ({}));
vi.mock('leaflet/dist/leaflet.css', () => ({}));
vi.mock('leaflet', () => ({
  default: {
    divIcon: () => ({}),
    Icon: { Default: { mergeOptions: () => {} } },
  },
}));

vi.mock('@/lib/api/nominatim', () => ({
  searchAddress: vi.fn(async () => [
    {
      place_id: 1,
      display_name: 'Москва, Тверская, 1',
      lat: '55.7558',
      lon: '37.6173',
    },
  ]),
  reverseGeocode: vi.fn(async () => 'Москва, Тверская, 1'),
}));

// Stable navigate mock so we can assert navigation post-submit.
const navigateMock = vi.fn();
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>(
    'react-router-dom',
  );
  return {
    ...actual,
    useNavigate: () => navigateMock,
  };
});

import { OrderCreatePage } from './OrderCreatePage';

const mockUser: User = {
  id: 'u-1',
  email: 'tester@example.com',
  first_name: 'Test',
  last_name: 'User',
  phone: '+79991111111',
  role: 'user',
};

describe('OrderCreatePage', () => {
  beforeEach(() => {
    navigateMock.mockReset();
  });

  it('renders heading "Создать заказ"', () => {
    renderWithProviders(<OrderCreatePage />, {
      route: '/orders/new',
      user: mockUser,
    });
    expect(
      screen.getByRole('heading', { name: 'Создать заказ' }),
    ).toBeInTheDocument();
  });

  it('submit button is disabled when no items + no address', () => {
    renderWithProviders(<OrderCreatePage />, {
      route: '/orders/new',
      user: mockUser,
    });
    // Two CartPanel instances render (desktop + mobile); both must be disabled.
    const buttons = screen.getAllByRole('button', { name: 'Оформить заказ' });
    expect(buttons.length).toBeGreaterThan(0);
    buttons.forEach((b) => expect(b).toBeDisabled());
  });

  it('phone field is prefilled from user.phone', async () => {
    renderWithProviders(<OrderCreatePage />, {
      route: '/orders/new',
      user: mockUser,
    });
    // Wait for menu data to load so the form is fully ready.
    await waitFor(() => {
      const phoneFields = screen.getAllByLabelText(
        'Телефон для связи',
      ) as HTMLInputElement[];
      expect(phoneFields.length).toBeGreaterThan(0);
      phoneFields.forEach((p) => expect(p.value).toBe('+79991111111'));
    });
  });

  it('selecting an address and adding an item enables submit', async () => {
    const user = userEvent.setup();
    renderWithProviders(<OrderCreatePage />, {
      route: '/orders/new',
      user: mockUser,
    });

    // Type address (>= 3 chars triggers MSW mock via mocked searchAddress)
    const addressInput = screen.getByPlaceholderText(
      'Поиск адреса в Москве...',
    );
    await user.type(addressInput, 'Тверская');

    // Click suggestion
    const suggestion = await screen.findByText(/Москва, Тверская, 1/);
    await user.click(suggestion);

    // Wait for menu items to load
    await waitFor(() => {
      expect(screen.getAllByLabelText('Увеличить количество').length).toBeGreaterThan(0);
    });

    // Increment first menu item
    const incButtons = screen.getAllByLabelText('Увеличить количество');
    await user.click(incButtons[0]);

    // Submit button no longer disabled (use desktop CartPanel — first match)
    await waitFor(() => {
      const submits = screen.getAllByRole('button', { name: 'Оформить заказ' });
      expect(submits.some((b) => !(b as HTMLButtonElement).disabled)).toBe(true);
    });
  });

  it('successful submission navigates to /orders/{id}', async () => {
    // Override createOrder MSW to deterministic id
    server.use(
      http.post('/v1/orders', () =>
        HttpResponse.json(
          {
            order: {
              id: 'ord-new',
              userId: 'u-1',
              status: 'ORDER_STATUS_CREATED',
              deliveryAddress: 'Москва, Тверская, 1',
              deliveryCoordinates: { latitude: 55.7558, longitude: 37.6173 },
              items: [{ name: 'Цезарь', quantity: 1, price: 390 }],
              contactPhone: '+79991111111',
              paymentMethod: 'card_on_delivery',
              createdAt: '2026-05-09T10:00:00Z',
              updatedAt: '2026-05-09T10:00:00Z',
            },
          },
          { status: 201 },
        ),
      ),
    );

    const user = userEvent.setup();
    renderWithProviders(<OrderCreatePage />, {
      route: '/orders/new',
      user: mockUser,
    });

    // Pick address
    const addressInput = screen.getByPlaceholderText(
      'Поиск адреса в Москве...',
    );
    await user.type(addressInput, 'Тверская');
    const suggestion = await screen.findByText(/Москва, Тверская, 1/);
    await user.click(suggestion);

    // Add item
    await waitFor(() => {
      expect(screen.getAllByLabelText('Увеличить количество').length).toBeGreaterThan(0);
    });
    const incButtons = screen.getAllByLabelText('Увеличить количество');
    await user.click(incButtons[0]);

    // Submit (desktop button — first match)
    await waitFor(() => {
      const submits = screen.getAllByRole('button', { name: 'Оформить заказ' });
      expect(submits.some((b) => !(b as HTMLButtonElement).disabled)).toBe(true);
    });
    const enabledSubmit = screen
      .getAllByRole('button', { name: 'Оформить заказ' })
      .find((b) => !(b as HTMLButtonElement).disabled)!;

    await act(async () => {
      await user.click(enabledSubmit);
    });

    await waitFor(() => {
      expect(navigateMock).toHaveBeenCalledWith('/orders/ord-new');
    });
  });
});
