-- 000005: Collapse all delivery zones down to the single canonical «Москва и МО».
-- Runtime test zones («Moscow Center», «Test Zone») are junk from prior test runs.
-- FK-safe: courier_zones.zone_id → delivery_zones(id) is ON DELETE NO ACTION, so
-- referencing rows MUST be repointed/removed BEFORE deleting the test zones.
-- Idempotent: every statement is independently re-runnable. Once only the canonical
-- zone remains, every step below is a no-op (empty source sets, NOT EXISTS guard).
-- Operate by name only — the canonical zone id is runtime-generated, never hardcoded.

-- Step 1: Ensure canonical zone exists (guard mirrors 000004; no-op if already present).
INSERT INTO delivery_zones (name, boundary)
SELECT 'Москва и МО',
       ST_GeomFromText('POLYGON((35.15 54.25, 40.20 54.25, 40.20 56.96, 35.15 56.96, 35.15 54.25))', 4326)
WHERE NOT EXISTS (
    SELECT 1 FROM delivery_zones WHERE name = 'Москва и МО'
);

-- Step 2a: Delete courier_zones rows that would PK-collide on repoint.
-- courier_zones PK is (courier_id, zone_id). A courier already holding the
-- canonical zone row AND a test-zone row would create a duplicate PK on UPDATE.
-- UPDATE does not support ON CONFLICT, so dedup by deleting the colliding test rows first.
DELETE FROM courier_zones cz
WHERE cz.zone_id IN (SELECT id FROM delivery_zones WHERE name <> 'Москва и МО')
  AND EXISTS (
      SELECT 1 FROM courier_zones c2
      WHERE c2.courier_id = cz.courier_id
        AND c2.zone_id = (SELECT id FROM delivery_zones WHERE name = 'Москва и МО')
  );

-- Step 2b: Repoint the surviving courier_zones test-zone rows to canonical
-- (no PK conflict possible now — collisions were removed in Step 2a).
UPDATE courier_zones
SET zone_id = (SELECT id FROM delivery_zones WHERE name = 'Москва и МО')
WHERE zone_id IN (SELECT id FROM delivery_zones WHERE name <> 'Москва и МО');

-- Step 3: Repoint deliveries.zone_id from test zones to canonical (NOT a FK;
-- done for data correctness so couriers still see these deliveries by zone).
-- NULL zone_id rows are intentionally left untouched (NULL is never IN any set).
UPDATE deliveries
SET zone_id = (SELECT id FROM delivery_zones WHERE name = 'Москва и МО')
WHERE zone_id IN (SELECT id FROM delivery_zones WHERE name <> 'Москва и МО');

-- Step 4: Delete the test zones. FK-safe now — no courier_zones row references them.
DELETE FROM delivery_zones
WHERE name <> 'Москва и МО';
