import { describe, it, expect } from 'vitest';
import { nominatimClient, searchAddress, reverseGeocode } from './nominatim';

describe('nominatim API client', () => {
  it('searchAddress returns results array with display_name', async () => {
    const results = await searchAddress('Тверская');
    expect(results).toHaveLength(1);
    expect(results[0].display_name).toBe('Москва, Тверская, 1');
    expect(results[0].lat).toBe('55.7558');
  });

  it('reverseGeocode returns display_name string', async () => {
    const display = await reverseGeocode(55.7558, 37.6173);
    expect(display).toBe('Москва, Тверская, 1');
  });

  it('nominatimClient has User-Agent header set', () => {
    const headers = nominatimClient.defaults.headers as unknown as Record<
      string,
      unknown
    >;
    expect(headers['User-Agent']).toBe('delivery-tracker-vkr/1.0');
  });

  it('nominatimClient does NOT have Authorization header by default', () => {
    const headers = nominatimClient.defaults.headers as unknown as Record<
      string,
      unknown
    >;
    expect(headers.Authorization).toBeUndefined();
  });
});
