-- DATA-02: one-time maintenance cleanup of load-test orders.
-- Deletes orders stuck in status='created' that have NO delivery (load-test junk:
-- ~213 442 such rows). Orders that have a delivery are preserved; non-'created'
-- orders are never touched. orders has zero inbound FK (verified) → FK-safe.
-- Idempotent: after cleanup no 'created' order without a delivery remains, so a
-- re-run deletes 0 rows.
DELETE FROM orders
WHERE status = 'created'
  AND id NOT IN (SELECT order_id FROM deliveries);

-- Reclaim disk (~59 MB). VACUUM FULL takes an exclusive lock on orders
-- (~seconds on the dev DB; only order-service touches orders, via gRPC).
-- Zero-lock fallback if the stack must stay hot: replace with `VACUUM orders;`
-- (marks pages reusable but does not return space to the OS).
VACUUM (FULL) orders;
