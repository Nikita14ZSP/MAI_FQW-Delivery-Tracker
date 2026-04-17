-- Reverse the "Москва и МО" service-area seed.
-- Delete dependent courier_zones rows FIRST (FK: courier_zones.zone_id ->
-- delivery_zones.id), then the zone itself. Scoped strictly to the seeded
-- zone name so ad-hoc zones ("Moscow Center" / "Test Zone") are untouched.

DELETE FROM courier_zones
WHERE zone_id IN (SELECT id FROM delivery_zones WHERE name = 'Москва и МО');

DELETE FROM delivery_zones
WHERE name = 'Москва и МО';
