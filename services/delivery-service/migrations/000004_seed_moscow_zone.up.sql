-- Service-area seed: ensures the system ships with one delivery zone covering
-- "Москва и МО" (canonical MOSCOW_REGION_BBOX: lat 54.25-56.96, lon 35.15-40.20).
-- Without this, a fresh DB has zero zones, so every order becomes a zone-less
-- pending delivery and no courier can ever be assigned.
-- Idempotent: the INSERT is guarded by NOT EXISTS on the zone name, so repeated
-- application (or re-runs against a DB that already has the zone) is a no-op.

INSERT INTO delivery_zones (name, boundary)
SELECT 'Москва и МО',
       ST_GeomFromText('POLYGON((35.15 54.25, 40.20 54.25, 40.20 56.96, 35.15 56.96, 35.15 54.25))', 4326)
WHERE NOT EXISTS (
    SELECT 1 FROM delivery_zones WHERE name = 'Москва и МО'
);
