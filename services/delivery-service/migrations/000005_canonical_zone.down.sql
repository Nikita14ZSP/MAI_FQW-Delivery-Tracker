-- 000005 collapses runtime test zones into canonical «Москва и МО».
-- The removed test zones had runtime-generated ids and unknown boundaries —
-- they are not restorable and are not part of the codebase. Down is an
-- intentional FK-free no-op so golang-migrate has a valid .down.sql that
-- never raises an FK (or any) error.
SELECT 1;
