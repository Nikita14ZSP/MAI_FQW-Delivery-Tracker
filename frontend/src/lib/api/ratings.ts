import { api } from '@/lib/api';

/** POST /v1/ratings response wire shape. grpc-gateway camelCase of rating_id. */
export interface RateCourierResponseRaw {
  ratingId: string;
}

/** Domain snake_case. */
export interface RatingResult {
  rating_id: string;
}

/** Normalize POST /v1/ratings response: { ratingId } → { rating_id }. */
export function normalizeRatingResult(raw: RateCourierResponseRaw): RatingResult {
  return { rating_id: raw.ratingId };
}

/**
 * POST /v1/ratings — submit a 1-5 star rating + optional comment for a delivery's courier.
 * Body uses proto snake_case field names (grpc-gateway body:"*"): { delivery_id, stars, comment }.
 * Response is camelCase { ratingId } → normalized to { rating_id }.
 * RBAC: POST /v1/ratings is NOT in RBACRules() → user-role JWT passes through (see 11-RESEARCH §RBAC).
 */
export async function submitRating(
  deliveryId: string,
  stars: number,
  comment: string,
): Promise<RatingResult> {
  const { data } = await api.post<RateCourierResponseRaw>('/ratings', {
    delivery_id: deliveryId,
    stars,
    comment,
  });
  return normalizeRatingResult(data);
}
