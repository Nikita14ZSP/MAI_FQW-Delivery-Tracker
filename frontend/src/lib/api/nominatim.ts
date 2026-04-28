import axios from 'axios';

const NOMINATIM_BASE = 'https://nominatim.openstreetmap.org';
const USER_AGENT = 'delivery-tracker-vkr/1.0';

export const nominatimClient = axios.create({
  baseURL: NOMINATIM_BASE,
  headers: { 'User-Agent': USER_AGENT },
  params: { format: 'json', 'accept-language': 'ru' },
});

export interface NominatimResult {
  place_id: number;
  display_name: string;
  lat: string;
  lon: string;
}

export async function searchAddress(query: string): Promise<NominatimResult[]> {
  const { data } = await nominatimClient.get<NominatimResult[]>('/search', {
    params: {
      q: query,
      countrycodes: 'ru',
      // viewbox order: lonMin,latMax,lonMax,latMin
      viewbox: '35.15,56.96,40.20,54.25',
      bounded: 1,
      limit: 5,
    },
  });
  return data;
}

export async function reverseGeocode(lat: number, lon: number): Promise<string> {
  const { data } = await nominatimClient.get<{ display_name: string }>(
    '/reverse',
    {
      params: { lat, lon },
    },
  );
  return data.display_name;
}
