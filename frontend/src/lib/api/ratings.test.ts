import { describe, it, expect } from 'vitest';
import { server } from '@/test/mocks/server';
import { http, HttpResponse } from 'msw';
import { submitRating, normalizeRatingResult } from './ratings';

describe('ratings API client', () => {
  it('submitRating returns normalized {rating_id} from camelCase {ratingId}', async () => {
    // relies on default ratingHandlers MSW handler returning { ratingId: 'rat-mock-1' }
    const result = await submitRating('dlv-mock-1', 4, 'Отличный курьер');
    expect(result).toEqual({ rating_id: 'rat-mock-1' });
  });

  it('submitRating POSTs body {delivery_id, stars, comment}', async () => {
    let captured: unknown;
    server.use(
      http.post('/v1/ratings', async ({ request }) => {
        captured = await request.json();
        return HttpResponse.json({ ratingId: 'rat-x' });
      }),
    );
    await submitRating('dlv-9', 3, 'ок');
    expect(captured).toEqual({ delivery_id: 'dlv-9', stars: 3, comment: 'ок' });
  });

  it('normalizeRatingResult maps ratingId → rating_id', () => {
    expect(normalizeRatingResult({ ratingId: 'rat-xyz' })).toEqual({ rating_id: 'rat-xyz' });
  });

  it('submitRating allows empty comment', async () => {
    server.use(
      http.post('/v1/ratings', async ({ request }) => {
        const b = await request.json() as Record<string, unknown>;
        void b;
        return HttpResponse.json({ ratingId: 'rat-e' });
      }),
    );
    await expect(submitRating('dlv-1', 5, '')).resolves.toEqual({ rating_id: 'rat-e' });
  });
});
