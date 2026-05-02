import { describe, it, expect, vi, beforeEach } from 'vitest';
import { createElement } from 'react';
import { render, screen, fireEvent } from '@testing-library/react';

// Mock react-leaflet per Phase 8 D-04 pattern (ESM createElement; vitest is ESM-first)
const mockMap = { fitBounds: vi.fn(), setView: vi.fn() };
const mockSetLatLng = vi.fn();

vi.mock('react-leaflet', () => ({
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  MapContainer: ({ children, ...props }: any) =>
    createElement('div', { 'data-testid': 'map-container', ...props }, children),
  TileLayer: () => createElement('div', { 'data-testid': 'tile-layer' }),
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  Marker: ({ position, ref }: any) => {
    if (typeof ref === 'function') {
      // Provide fake L.Marker with setLatLng spy
      ref({ setLatLng: mockSetLatLng });
    }
    return createElement('div', {
      'data-testid': 'marker',
      'data-lat': position[0],
      'data-lng': position[1],
    });
  },
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  Polyline: ({ positions }: any) =>
    createElement('div', {
      'data-testid': 'polyline',
      'data-count': positions.length,
    }),
  useMap: () => mockMap,
}));

// Mock leaflet-icon-fix side effect (PNG imports jsdom can't process)
vi.mock('@/lib/leaflet-icon-fix', () => ({}));
vi.mock('leaflet/dist/leaflet.css', () => ({}));
vi.mock('leaflet', () => ({
  default: {
    divIcon: () => ({}),
    Icon: { Default: { mergeOptions: () => {} } },
  },
}));

import { TrackingMap } from './TrackingMap';

const destination = { lat: 55.76, lng: 37.62 };

beforeEach(() => {
  mockMap.fitBounds.mockClear();
  mockMap.setView.mockClear();
  mockSetLatLng.mockClear();
});

describe('TrackingMap', () => {
  it('renders MapContainer + TileLayer + destination Marker even when no courier', () => {
    render(<TrackingMap destination={destination} lastLocation={null} history={[]} />);
    expect(screen.getByTestId('map-container')).toBeInTheDocument();
    expect(screen.getByTestId('tile-layer')).toBeInTheDocument();
    expect(screen.getAllByTestId('marker')).toHaveLength(1); // destination only
  });

  it('renders both markers when lastLocation is present', () => {
    render(
      <TrackingMap
        destination={destination}
        lastLocation={{
          id: '',
          courier_id: 'c',
          order_id: 'o',
          lat: 55.7,
          lng: 37.6,
          recorded_at: 't',
        }}
        history={[]}
      />,
    );
    expect(screen.getAllByTestId('marker')).toHaveLength(2);
  });

  it('renders Polyline when history.length > 1', () => {
    const history = [
      { id: '', courier_id: 'c', order_id: 'o', lat: 55.7, lng: 37.6, recorded_at: 't1' },
      { id: '', courier_id: 'c', order_id: 'o', lat: 55.71, lng: 37.61, recorded_at: 't2' },
    ];
    render(
      <TrackingMap destination={destination} lastLocation={history[1]} history={history} />,
    );
    expect(screen.getByTestId('polyline')).toBeInTheDocument();
    expect(screen.getByTestId('polyline')).toHaveAttribute('data-count', '2');
  });

  it('does NOT render Polyline when history.length <= 1', () => {
    render(<TrackingMap destination={destination} lastLocation={null} history={[]} />);
    expect(screen.queryByTestId('polyline')).toBeNull();
  });

  it('imperatively updates courier marker position when lastLocation changes', () => {
    // Install synchronous rAF stub that fires with elapsed > duration so t=1 immediately
    // (PLSH-01: second fix uses rAF tween; stub completes tween in one synchronous call)
    const rafStub = vi.fn((cb: FrameRequestCallback) => {
      cb(performance.now() + 10000); // elapsed >> 5000ms duration → t=1 (no recursion)
      return 1;
    });
    vi.stubGlobal('requestAnimationFrame', rafStub);
    vi.stubGlobal('cancelAnimationFrame', vi.fn());

    const initial = {
      id: '',
      courier_id: 'c',
      order_id: 'o',
      lat: 55.7,
      lng: 37.6,
      recorded_at: 't1',
    };
    const { rerender } = render(
      <TrackingMap destination={destination} lastLocation={initial} history={[initial]} />,
    );
    mockSetLatLng.mockClear();
    rerender(
      <TrackingMap
        destination={destination}
        lastLocation={{ ...initial, lat: 55.8, lng: 37.7, recorded_at: 't2' }}
        history={[initial]}
      />,
    );
    expect(mockSetLatLng).toHaveBeenCalledWith([55.8, 37.7]);

    vi.unstubAllGlobals();
  });

  it('fits bounds once on initial load with courier', () => {
    const courier = {
      id: '',
      courier_id: 'c',
      order_id: 'o',
      lat: 55.7,
      lng: 37.6,
      recorded_at: 't',
    };
    render(<TrackingMap destination={destination} lastLocation={courier} history={[courier]} />);
    expect(mockMap.fitBounds).toHaveBeenCalledTimes(1);
    expect(mockMap.fitBounds).toHaveBeenCalledWith(
      [
        [destination.lat, destination.lng],
        [courier.lat, courier.lng],
      ],
      { padding: [50, 50] },
    );
  });

  it('recenter button click triggers fitBounds', () => {
    const courier = {
      id: '',
      courier_id: 'c',
      order_id: 'o',
      lat: 55.7,
      lng: 37.6,
      recorded_at: 't',
    };
    render(<TrackingMap destination={destination} lastLocation={courier} history={[courier]} />);
    mockMap.fitBounds.mockClear();
    fireEvent.click(screen.getByLabelText('Центрировать карту'));
    expect(mockMap.fitBounds).toHaveBeenCalledTimes(1);
  });

  // --- quick-260516-e30: graceful degradation, no white-screen on bad data ---
  it('does not throw and excludes malformed points from the Polyline', () => {
    const valid1 = {
      id: '',
      courier_id: 'c',
      order_id: 'o',
      lat: 55.7,
      lng: 37.6,
      recorded_at: 't1',
    };
    const malformed = {
      id: '',
      courier_id: 'c',
      order_id: 'o',
      lat: undefined as unknown as number,
      lng: undefined as unknown as number,
      recorded_at: 't2',
    };
    const valid2 = {
      id: '',
      courier_id: 'c',
      order_id: 'o',
      lat: 55.71,
      lng: 37.61,
      recorded_at: 't3',
    };
    expect(() =>
      render(
        <TrackingMap
          destination={destination}
          lastLocation={null}
          history={[valid1, malformed, valid2]}
        />,
      ),
    ).not.toThrow();
    expect(screen.getByTestId('map-container')).toBeInTheDocument();
    // Polyline keeps only the 2 finite points (malformed dropped)
    expect(screen.getByTestId('polyline')).toHaveAttribute('data-count', '2');
  });

  it('skips courier marker / setLatLng when lastLocation is not finite', () => {
    const bad = {
      id: '',
      courier_id: 'c',
      order_id: 'o',
      lat: NaN,
      lng: NaN,
      recorded_at: 't',
    };
    expect(() =>
      render(
        <TrackingMap destination={destination} lastLocation={bad} history={[]} />,
      ),
    ).not.toThrow();
    // Only the destination marker — courier marker skipped (non-finite)
    expect(screen.getAllByTestId('marker')).toHaveLength(1);
    expect(mockSetLatLng).not.toHaveBeenCalled();
  });
});

// PLSH-01 rAF tween — unit tests for smooth courier marker animation
describe('TrackingMap — PLSH-01 rAF tween', () => {
  const fix1 = {
    id: '',
    courier_id: 'c',
    order_id: 'o',
    lat: 55.70,
    lng: 37.60,
    recorded_at: 't1',
  };
  const fix2 = {
    id: '',
    courier_id: 'c',
    order_id: 'o',
    lat: 55.80,
    lng: 37.70,
    recorded_at: 't2',
  };
  const fix3 = {
    id: '',
    courier_id: 'c',
    order_id: 'o',
    lat: 55.90,
    lng: 37.80,
    recorded_at: 't3',
  };

  // Test A: first GPS fix is instant placement, NO requestAnimationFrame
  it('places marker instantly on first GPS fix — no rAF called (D-09)', () => {
    const rafSpy = vi.spyOn(window, 'requestAnimationFrame');
    render(
      <TrackingMap destination={destination} lastLocation={fix1} history={[]} />,
    );
    expect(mockSetLatLng).toHaveBeenCalledWith([55.70, 37.60]);
    expect(rafSpy).not.toHaveBeenCalled();
    rafSpy.mockRestore();
  });

  // Test B: second GPS fix triggers rAF tween; synchronous stub completes tween immediately
  it('starts rAF tween on second GPS fix and reaches target coords (D-07..D-08)', () => {
    // Pass now+10000 so elapsed >> 5000ms duration → t=1 immediately (no recursion)
    const rafStub = vi.fn((cb: FrameRequestCallback) => {
      cb(performance.now() + 10000);
      return 1;
    });
    const cancelStub = vi.fn();
    vi.stubGlobal('requestAnimationFrame', rafStub);
    vi.stubGlobal('cancelAnimationFrame', cancelStub);

    const { rerender } = render(
      <TrackingMap destination={destination} lastLocation={fix1} history={[]} />,
    );
    mockSetLatLng.mockClear();
    rerender(
      <TrackingMap destination={destination} lastLocation={fix2} history={[]} />,
    );

    expect(rafStub).toHaveBeenCalled();
    // Synchronous stub fires cb(performance.now()) immediately — elapsed is huge → t clamped to 1
    // So final setLatLng call should be at fix2 coords
    expect(mockSetLatLng).toHaveBeenLastCalledWith([55.80, 37.70]);

    vi.unstubAllGlobals();
  });

  // Test C: third GPS fix mid-tween causes cancelAnimationFrame to be called
  it('cancels in-flight tween when a new GPS fix arrives (D-08)', () => {
    // Use a stub that does NOT fire the callback — keeps tween in-flight
    // so that when fix3 arrives, cancelAnimationFrame is called before a new tween starts
    const rafStub = vi.fn((_cb: FrameRequestCallback) => {
      return 99; // Return non-null id; do NOT fire cb — tween stays in flight
    });
    const cancelStub = vi.fn();
    vi.stubGlobal('requestAnimationFrame', rafStub);
    vi.stubGlobal('cancelAnimationFrame', cancelStub);

    const { rerender } = render(
      <TrackingMap destination={destination} lastLocation={fix1} history={[]} />,
    );
    // fix2: rafStub fires → tween in-flight (cb not called → tweenRef.current = 99)
    rerender(
      <TrackingMap destination={destination} lastLocation={fix2} history={[]} />,
    );
    // fix3: useEffect cleanup runs → cancelAnimationFrame(99) called; new tween starts
    rerender(
      <TrackingMap destination={destination} lastLocation={fix3} history={[]} />,
    );

    expect(cancelStub).toHaveBeenCalled();

    vi.unstubAllGlobals();
  });

  // Test D: unmount with an in-flight tween cancels it (no post-unmount setLatLng error)
  it('cancels in-flight tween on unmount — cleanup calls cancelAnimationFrame (Pitfall 1)', () => {
    // Use a stub that does NOT fire the callback synchronously so tween stays in-flight
    // when unmount is called — this ensures cancelAnimationFrame runs from cleanup
    const rafStub = vi.fn((_cb: FrameRequestCallback) => {
      // Do NOT call cb — tween stays in-flight
      return 42; // Return non-null id so cleanup has something to cancel
    });
    const cancelStub = vi.fn();
    vi.stubGlobal('requestAnimationFrame', rafStub);
    vi.stubGlobal('cancelAnimationFrame', cancelStub);

    const { rerender, unmount } = render(
      <TrackingMap destination={destination} lastLocation={fix1} history={[]} />,
    );
    rerender(
      <TrackingMap destination={destination} lastLocation={fix2} history={[]} />,
    );
    unmount();

    expect(cancelStub).toHaveBeenCalled();

    vi.unstubAllGlobals();
  });

  // Test E: regression guard — Polyline still renders with correct count when history > 1
  it('renders Polyline with correct data-count when history has 3 finite points (regression guard)', () => {
    const history = [
      { id: '', courier_id: 'c', order_id: 'o', lat: 55.70, lng: 37.60, recorded_at: 't1' },
      { id: '', courier_id: 'c', order_id: 'o', lat: 55.75, lng: 37.65, recorded_at: 't2' },
      { id: '', courier_id: 'c', order_id: 'o', lat: 55.80, lng: 37.70, recorded_at: 't3' },
    ];
    render(
      <TrackingMap destination={destination} lastLocation={history[2]} history={history} />,
    );
    expect(screen.getByTestId('polyline').getAttribute('data-count')).toBe('3');
  });
});
