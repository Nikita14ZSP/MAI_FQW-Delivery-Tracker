import { useEffect } from 'react';
import {
  MapContainer,
  TileLayer,
  Marker,
  useMap,
  useMapEvents,
} from 'react-leaflet';
import L from 'leaflet';
import 'leaflet/dist/leaflet.css';
import '@/lib/leaflet-icon-fix';
import { reverseGeocode } from '@/lib/api/nominatim';

const MOSCOW_CENTER: [number, number] = [55.7558, 37.6173];

// Module-level singleton: the icon has no dynamic inputs, so one shared
// instance avoids re-creating the divIcon (and replaying pinDrop) on every
// parent re-render. The animation still plays once on first mount.
const PIN_ICON = L.divIcon({
  className: '',
  html: `<div style="
      width:24px;height:36px;background:#2563EB;border:2px solid white;
      border-radius:50% 50% 50% 0;transform:rotate(-45deg);
      box-shadow:0 2px 8px rgba(0,0,0,0.25);
      animation:pinDrop 200ms ease-out;
    " title="Перетащите маркер для уточнения адреса"></div>`,
  iconSize: [24, 36],
  iconAnchor: [12, 36],
  popupAnchor: [0, -36],
});

export interface MapPickerPosition {
  lat: number;
  lon: number;
  displayName: string;
}

export interface MapPickerProps {
  position: MapPickerPosition | null;
  onPositionChange: (next: MapPickerPosition) => void;
}

function MapClickHandler({
  onPositionChange,
}: {
  onPositionChange: MapPickerProps['onPositionChange'];
}) {
  useMapEvents({
    async click(e) {
      const lat = e.latlng.lat;
      const lon = e.latlng.lng;
      let displayName = '';
      try {
        displayName = await reverseGeocode(lat, lon);
      } catch {
        // network failure → leave empty; caller may revalidate
      }
      onPositionChange({ lat, lon, displayName });
    },
  });
  return null;
}

function FlyToHandler({ position }: { position: MapPickerPosition | null }) {
  const map = useMap();
  useEffect(() => {
    if (position) {
      map.flyTo([position.lat, position.lon], 15, { duration: 1 });
    }
  }, [position?.lat, position?.lon, map]);
  return null;
}

export function MapPicker({ position, onPositionChange }: MapPickerProps) {
  return (
    <div role="application" aria-label="Выбор адреса на карте">
    <MapContainer
      center={position ? [position.lat, position.lon] : MOSCOW_CENTER}
      zoom={11}
      minZoom={9}
      maxZoom={18}
      attributionControl={false}
      className="h-[320px] lg:h-[400px] rounded-lg overflow-hidden border border-gray-200"
    >
      <TileLayer url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png" />
      <MapClickHandler onPositionChange={onPositionChange} />
      <FlyToHandler position={position} />
      {position && (
        <Marker
          position={[position.lat, position.lon]}
          draggable
          icon={PIN_ICON}
          eventHandlers={{
            async dragend(e) {
              const m = e.target as L.Marker;
              const { lat, lng } = m.getLatLng();
              let displayName = '';
              try {
                displayName = await reverseGeocode(lat, lng);
              } catch {
                // swallow
              }
              onPositionChange({ lat, lon: lng, displayName });
            },
          }}
        />
      )}
    </MapContainer>
    </div>
  );
}
