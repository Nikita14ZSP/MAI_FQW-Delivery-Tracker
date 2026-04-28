import { describe, it, expect, vi } from 'vitest';
import { createElement, type ReactNode } from 'react';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

// Mock react-leaflet primitives BEFORE importing MapPicker.
// jsdom cannot compute layout for Leaflet; we use simple DOM stand-ins.
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
  TileLayer: () =>
    createElement('div', { 'data-testid': 'tile-layer' }),
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  Marker: ({ position, eventHandlers }: any) =>
    createElement('button', {
      'data-testid': 'pin-marker',
      'data-lat': position[0],
      'data-lon': position[1],
      type: 'button',
      onClick: () =>
        eventHandlers?.dragend?.({
          target: {
            getLatLng: () => ({
              lat: position[0] + 0.01,
              lng: position[1] + 0.01,
            }),
          },
        }),
    }),
  useMap: () => ({ flyTo: vi.fn() }),
  useMapEvents: () => null,
}));

// Mock leaflet-icon-fix side effect import (loads PNG assets that jsdom can't process).
vi.mock('@/lib/leaflet-icon-fix', () => ({}));
vi.mock('leaflet/dist/leaflet.css', () => ({}));
vi.mock('leaflet', () => ({
  default: {
    divIcon: () => ({}),
    Icon: { Default: { mergeOptions: () => {} } },
  },
}));

// Mock reverseGeocode to avoid network in dragend handler test.
vi.mock('@/lib/api/nominatim', () => ({
  reverseGeocode: vi.fn(async () => 'Москва, новый адрес'),
}));

import { MapPicker } from './MapPicker';
import { reverseGeocode } from '@/lib/api/nominatim';

describe('MapPicker', () => {
  it('renders MapContainer with role application and aria-label', () => {
    render(<MapPicker position={null} onPositionChange={vi.fn()} />);
    expect(
      screen.getByRole('application', { name: 'Выбор адреса на карте' }),
    ).toBeInTheDocument();
  });

  it('does not render marker when position is null', () => {
    render(<MapPicker position={null} onPositionChange={vi.fn()} />);
    expect(screen.queryByTestId('pin-marker')).not.toBeInTheDocument();
  });

  it('renders marker at given position', () => {
    render(
      <MapPicker
        position={{ lat: 55.7, lon: 37.6, displayName: '' }}
        onPositionChange={vi.fn()}
      />,
    );
    const marker = screen.getByTestId('pin-marker');
    expect(marker).toBeInTheDocument();
    expect(marker.getAttribute('data-lat')).toBe('55.7');
    expect(marker.getAttribute('data-lon')).toBe('37.6');
  });

  it('dragend triggers reverseGeocode and calls onPositionChange', async () => {
    const onPositionChange = vi.fn();
    const user = userEvent.setup();
    render(
      <MapPicker
        position={{ lat: 55.7, lon: 37.6, displayName: '' }}
        onPositionChange={onPositionChange}
      />,
    );
    await user.click(screen.getByTestId('pin-marker'));

    await waitFor(() => {
      expect(reverseGeocode).toHaveBeenCalled();
      expect(onPositionChange).toHaveBeenCalledTimes(1);
    });
    const call = onPositionChange.mock.calls[0]?.[0] as {
      lat: number;
      lon: number;
      displayName: string;
    };
    expect(call.lat).toBeCloseTo(55.71, 5);
    expect(call.lon).toBeCloseTo(37.61, 5);
    expect(call.displayName).toBe('Москва, новый адрес');
  });
});
