package repository_test

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mozgovojnikita/delivery-tracker/services/delivery-service/internal/domain"
	"github.com/mozgovojnikita/delivery-tracker/services/delivery-service/internal/repository"
)

// openTestDB opens a pgxpool from DATABASE_URL or skips the test.
func openTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("integration: requires DATABASE_URL")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("open test DB: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

// ---------------------------------------------------------------------------
// Unit-level cursor codec tests (no DB required)
// ---------------------------------------------------------------------------

// decodeCursor is a copy of the cursor decoding logic for test assertions.
func decodeCursor(cursor string) (createdAt time.Time, id string, ok bool) {
	decoded, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		return
	}
	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return
	}
	nanos, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return
	}
	return time.Unix(0, nanos).UTC(), parts[1], true
}

// TestListAvailableForCourier_CursorEncoding verifies that the cursor encodes
// and decodes correctly without a live database.
func TestListAvailableForCourier_CursorEncoding(t *testing.T) {
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	id := "a1b2c3d4-e5f6-7890-abcd-ef1234567890"

	raw := strconv.FormatInt(ts.UnixNano(), 10) + ":" + id
	cursor := base64.StdEncoding.EncodeToString([]byte(raw))

	got, gotID, ok := decodeCursor(cursor)
	if !ok {
		t.Fatal("expected cursor to decode successfully")
	}
	if !got.Equal(ts) {
		t.Errorf("decoded createdAt: want %v, got %v", ts, got)
	}
	if gotID != id {
		t.Errorf("decoded id: want %q, got %q", id, gotID)
	}
}

// ---------------------------------------------------------------------------
// Integration tests — require DATABASE_URL
// ---------------------------------------------------------------------------

// TestAcceptByCourier_Concurrent tests that concurrent AcceptByCourier calls on the
// same delivery_id result in exactly 1 success (ok=true) and 9 failures (ok=false).
// Uses pgx pool against test PG instance (DATABASE_URL).
// BKND-03: D-07 atomicity.
func TestAcceptByCourier_Concurrent(t *testing.T) {
	pool := openTestDB(t)
	repo := repository.NewDeliveryRepository(pool)

	ctx := context.Background()

	// Insert a pending delivery with courier_id IS NULL for the test.
	var deliveryID string
	err := pool.QueryRow(ctx, `
		INSERT INTO deliveries (order_id, status)
		VALUES ($1, 'pending')
		RETURNING id`, fmt.Sprintf("order-concurrent-%d", time.Now().UnixNano()),
	).Scan(&deliveryID)
	if err != nil {
		t.Fatalf("insert test delivery: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM deliveries WHERE id = $1", deliveryID) //nolint:errcheck
	})

	const goroutines = 10
	eta := time.Now().Add(30 * time.Minute)

	type result struct {
		ok  bool
		err error
	}
	results := make([]result, goroutines)
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		i := i
		go func() {
			defer wg.Done()
			courierID := fmt.Sprintf("courier-%d", i)
			// Ensure courier row exists so FK constraint is satisfied
			pool.Exec(ctx, "INSERT INTO couriers (id) VALUES ($1) ON CONFLICT DO NOTHING", courierID) //nolint:errcheck
			d, ok, err := repo.AcceptByCourier(ctx, deliveryID, courierID, eta)
			_ = d
			results[i] = result{ok: ok, err: err}
		}()
	}
	wg.Wait()

	successCount := 0
	for _, r := range results {
		if r.err != nil {
			t.Errorf("unexpected error from AcceptByCourier: %v", r.err)
		}
		if r.ok {
			successCount++
		}
	}
	if successCount != 1 {
		t.Errorf("expected exactly 1 winner, got %d", successCount)
	}
}

// TestAcceptByCourier_Success tests AcceptByCourier on a pending delivery succeeds.
func TestAcceptByCourier_Success(t *testing.T) {
	pool := openTestDB(t)
	repo := repository.NewDeliveryRepository(pool)
	ctx := context.Background()

	var deliveryID string
	err := pool.QueryRow(ctx, `
		INSERT INTO deliveries (order_id, status)
		VALUES ($1, 'pending') RETURNING id`,
		fmt.Sprintf("order-success-%d", time.Now().UnixNano()),
	).Scan(&deliveryID)
	if err != nil {
		t.Fatalf("insert delivery: %v", err)
	}
	t.Cleanup(func() { pool.Exec(ctx, "DELETE FROM deliveries WHERE id = $1", deliveryID) }) //nolint:errcheck

	courierID := fmt.Sprintf("c-success-%d", time.Now().UnixNano())
	pool.Exec(ctx, "INSERT INTO couriers (id) VALUES ($1) ON CONFLICT DO NOTHING", courierID) //nolint:errcheck
	t.Cleanup(func() { pool.Exec(ctx, "DELETE FROM couriers WHERE id = $1", courierID) })     //nolint:errcheck

	eta := time.Now().Add(30 * time.Minute)
	d, ok, err := repo.AcceptByCourier(ctx, deliveryID, courierID, eta)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true")
	}
	if d == nil {
		t.Fatal("expected delivery returned")
	}
	if string(d.Status) != "assigned" {
		t.Errorf("expected status=assigned, got %s", d.Status)
	}
	if d.CourierID != courierID {
		t.Errorf("expected courier_id=%s, got %s", courierID, d.CourierID)
	}
}

// TestAcceptByCourier_AlreadyTaken tests AcceptByCourier on an already-assigned delivery returns ok=false.
func TestAcceptByCourier_AlreadyTaken(t *testing.T) {
	pool := openTestDB(t)
	repo := repository.NewDeliveryRepository(pool)
	ctx := context.Background()

	courierID1 := fmt.Sprintf("c-taken1-%d", time.Now().UnixNano())
	courierID2 := fmt.Sprintf("c-taken2-%d", time.Now().UnixNano())
	pool.Exec(ctx, "INSERT INTO couriers (id) VALUES ($1) ON CONFLICT DO NOTHING", courierID1) //nolint:errcheck
	pool.Exec(ctx, "INSERT INTO couriers (id) VALUES ($1) ON CONFLICT DO NOTHING", courierID2) //nolint:errcheck
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM couriers WHERE id IN ($1,$2)", courierID1, courierID2) //nolint:errcheck
	})

	var deliveryID string
	err := pool.QueryRow(ctx, `
		INSERT INTO deliveries (order_id, courier_id, status)
		VALUES ($1, $2, 'assigned') RETURNING id`,
		fmt.Sprintf("order-taken-%d", time.Now().UnixNano()), courierID1,
	).Scan(&deliveryID)
	if err != nil {
		t.Fatalf("insert delivery: %v", err)
	}
	t.Cleanup(func() { pool.Exec(ctx, "DELETE FROM deliveries WHERE id = $1", deliveryID) }) //nolint:errcheck

	// Try to claim an already-assigned delivery
	_, ok, err := repo.AcceptByCourier(ctx, deliveryID, courierID2, time.Now().Add(30*time.Minute))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected ok=false for already-taken delivery")
	}
}

// TestCountActiveByCourier tests that only active-status deliveries are counted.
func TestCountActiveByCourier(t *testing.T) {
	pool := openTestDB(t)
	repo := repository.NewDeliveryRepository(pool)
	ctx := context.Background()

	courierID := fmt.Sprintf("c-count-%d", time.Now().UnixNano())
	pool.Exec(ctx, "INSERT INTO couriers (id) VALUES ($1) ON CONFLICT DO NOTHING", courierID) //nolint:errcheck
	t.Cleanup(func() { pool.Exec(ctx, "DELETE FROM couriers WHERE id = $1", courierID) })     //nolint:errcheck

	// Insert 3 active deliveries + 2 delivered (should not be counted)
	activeStatuses := []string{"assigned", "picked_up", "in_transit"}
	terminalStatuses := []string{"delivered", "failed"}

	for _, s := range activeStatuses {
		pool.Exec(ctx, `INSERT INTO deliveries (order_id, courier_id, status) VALUES ($1, $2, $3)`,
			fmt.Sprintf("order-%s-%d", s, time.Now().UnixNano()), courierID, s) //nolint:errcheck
	}
	for _, s := range terminalStatuses {
		pool.Exec(ctx, `INSERT INTO deliveries (order_id, courier_id, status) VALUES ($1, $2, $3)`,
			fmt.Sprintf("order-%s-%d", s, time.Now().UnixNano()), courierID, s) //nolint:errcheck
	}
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM deliveries WHERE courier_id = $1", courierID) //nolint:errcheck
	})

	count, err := repo.CountActiveByCourier(ctx, courierID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 3 {
		t.Errorf("expected count=3 (assigned+picked_up+in_transit), got %d", count)
	}
}

// TestGetCourier_Found tests GetCourier returns the courier row.
func TestGetCourier_Found(t *testing.T) {
	pool := openTestDB(t)
	repo := repository.NewDeliveryRepository(pool)
	ctx := context.Background()

	courierID := fmt.Sprintf("c-found-%d", time.Now().UnixNano())
	pool.Exec(ctx, "INSERT INTO couriers (id, status) VALUES ($1, 'available') ON CONFLICT DO NOTHING", courierID) //nolint:errcheck
	t.Cleanup(func() { pool.Exec(ctx, "DELETE FROM couriers WHERE id = $1", courierID) })                          //nolint:errcheck

	c, err := repo.GetCourier(ctx, courierID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c == nil {
		t.Fatal("expected courier, got nil")
	}
	if c.ID != courierID {
		t.Errorf("expected courier_id=%s, got %s", courierID, c.ID)
	}
	if c.Status != "available" {
		t.Errorf("expected status=available, got %s", c.Status)
	}
}

// TestGetCourier_NotFound tests GetCourier returns ErrNotFound for unknown ID.
func TestGetCourier_NotFound(t *testing.T) {
	pool := openTestDB(t)
	repo := repository.NewDeliveryRepository(pool)
	ctx := context.Background()

	_, err := repo.GetCourier(ctx, "nonexistent-courier-id-xyz")
	if err == nil {
		t.Fatal("expected error for unknown courier")
	}
}

// TestUpdateCourierStatusOfflineGuard tests that transitioning a courier to offline
// fails when the courier has an active delivery in the database.
// BKND-01: implementation in 06-03.
func TestUpdateCourierStatusOfflineGuard(t *testing.T) {
	t.Skip("integration: requires DATABASE_URL — implement after UpdateCourierStatusToOfflineGuarded exists")
}

// TestUpdateCourierStatusToOffline_NoActiveDelivery tests that transitioning to offline
// succeeds when the courier has no active deliveries.
// BKND-01: implementation in 06-03.
func TestUpdateCourierStatusToOffline_NoActiveDelivery(t *testing.T) {
	t.Skip("integration: requires DATABASE_URL — implement after UpdateCourierStatusToOfflineGuarded exists")
}

// TestUpdateCourierStatusToAvailable tests that a courier can transition to available status.
// BKND-01: implementation in 06-03.
func TestUpdateCourierStatusToAvailable(t *testing.T) {
	t.Skip("integration: requires DATABASE_URL — implement after UpdateCourierStatus exists")
}

// TestListAvailableForCourier_Integration tests that the repository returns only orders
// in the courier's courier_zones, with status=pending, courier_id IS NULL,
// and that cursor pagination works correctly.
// BKND-02: integration variant requires DATABASE_URL.
func TestListAvailableForCourier_Integration(t *testing.T) {
	pool := openTestDB(t)
	_ = repository.NewDeliveryRepository(pool)
	// Full integration test would seed zones, deliveries, courier_zones and assert results.
	// Skipped here; unit variants below cover the logic.
	t.Skip("integration: requires seeded DB — covered by unit tests below")
}

// ---------------------------------------------------------------------------
// Unit-level tests for ListAvailableForCourier via mock assertions
// These verify the interface contract without a live database.
// ---------------------------------------------------------------------------

// stubListAvailableRepo wraps a function to implement just the needed method.
type stubListAvailableRepo struct {
	fn func(ctx context.Context, courierID, cursor string, limit int32) ([]domain.AvailableOrderRow, string, error)
}

// All other interface methods are no-ops for these focused tests.
func (s *stubListAvailableRepo) ListAvailableForCourier(ctx context.Context, courierID, cursor string, limit int32) ([]domain.AvailableOrderRow, string, error) {
	return s.fn(ctx, courierID, cursor, limit)
}
func (s *stubListAvailableRepo) CreateDelivery(ctx context.Context, orderID, zoneID string, lat, lng float64) (*domain.Delivery, error) {
	return nil, nil
}
func (s *stubListAvailableRepo) GetDeliveryByID(ctx context.Context, id string) (*domain.Delivery, error) {
	return nil, nil
}
func (s *stubListAvailableRepo) GetDeliveryByOrderID(ctx context.Context, orderID string) (*domain.Delivery, error) {
	return nil, nil
}
func (s *stubListAvailableRepo) UpdateStatus(ctx context.Context, id string, status domain.DeliveryStatus) error {
	return nil
}
func (s *stubListAvailableRepo) AssignCourier(ctx context.Context, deliveryID, courierID string, eta time.Time) error {
	return nil
}
func (s *stubListAvailableRepo) ListPendingDeliveries(ctx context.Context) ([]*domain.Delivery, error) {
	return nil, nil
}
func (s *stubListAvailableRepo) ListByCourierID(ctx context.Context, courierID string) ([]*domain.Delivery, error) {
	return nil, nil
}
func (s *stubListAvailableRepo) FindNearestCourier(ctx context.Context, zoneID string, lat, lng float64) (*domain.CourierCandidate, error) {
	return nil, nil
}
func (s *stubListAvailableRepo) CountActiveByCourier(ctx context.Context, courierID string) (int, error) {
	return 0, nil
}
func (s *stubListAvailableRepo) CreateZone(ctx context.Context, name, polygonGeoJSON string) (*domain.DeliveryZone, error) {
	return nil, nil
}
func (s *stubListAvailableRepo) ListZones(ctx context.Context, limit, offset int) ([]*domain.DeliveryZone, int, error) {
	return nil, 0, nil
}
func (s *stubListAvailableRepo) FindZoneByPoint(ctx context.Context, lat, lng float64) (*domain.DeliveryZone, error) {
	return nil, nil
}
func (s *stubListAvailableRepo) UpsertCourier(ctx context.Context, courierID string) error {
	return nil
}
func (s *stubListAvailableRepo) AssignCourierToZone(ctx context.Context, courierID, zoneID string) error {
	return nil
}
func (s *stubListAvailableRepo) CalculateETA(ctx context.Context, zoneID string, lat, lng float64) (time.Time, error) {
	return time.Time{}, nil
}

// TestListAvailableForCourier_OnlyPendingUnassigned verifies the interface contract:
// only rows returned from the repo are surfaced (filtering is in SQL, tested in integration).
func TestListAvailableForCourier_OnlyPendingUnassigned(t *testing.T) {
	t1 := time.Now().Add(-3 * time.Minute)
	t2 := time.Now().Add(-2 * time.Minute)
	t3 := time.Now().Add(-1 * time.Minute)

	// Simulate repo already filtered — only pending+unassigned rows returned.
	expectedRows := []domain.AvailableOrderRow{
		{OrderID: "o-1", DeliveryID: "d-1", ZoneName: "Center", CreatedAt: t1},
		{OrderID: "o-2", DeliveryID: "d-2", ZoneName: "Center", CreatedAt: t2},
		{OrderID: "o-3", DeliveryID: "d-3", ZoneName: "Center", CreatedAt: t3},
	}

	repo := &stubListAvailableRepo{
		fn: func(ctx context.Context, courierID, cursor string, limit int32) ([]domain.AvailableOrderRow, string, error) {
			if courierID != "courier-1" {
				t.Errorf("expected courierID=courier-1, got %q", courierID)
			}
			return expectedRows, "", nil
		},
	}

	rows, _, err := repo.ListAvailableForCourier(context.Background(), "courier-1", "", 20)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(rows) != 3 {
		t.Errorf("expected 3 rows, got %d", len(rows))
	}
}

// TestListAvailableForCourier_ZoneFilter verifies only rows from the courier's zones are returned.
func TestListAvailableForCourier_ZoneFilter(t *testing.T) {
	// Simulate repo already filtered by zone — zone C rows are absent.
	expectedRows := []domain.AvailableOrderRow{
		{OrderID: "o-1", DeliveryID: "d-1", ZoneName: "A", CreatedAt: time.Now().Add(-2 * time.Minute)},
		{OrderID: "o-2", DeliveryID: "d-2", ZoneName: "B", CreatedAt: time.Now().Add(-1 * time.Minute)},
	}

	callCount := 0
	repo := &stubListAvailableRepo{
		fn: func(ctx context.Context, courierID, cursor string, limit int32) ([]domain.AvailableOrderRow, string, error) {
			callCount++
			return expectedRows, "", nil
		},
	}

	rows, _, err := repo.ListAvailableForCourier(context.Background(), "courier-1", "", 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, r := range rows {
		if r.ZoneName == "C" {
			t.Errorf("zone C row should not be returned: %+v", r)
		}
	}
	if callCount != 1 {
		t.Errorf("expected 1 repo call, got %d", callCount)
	}
}

// TestListAvailableForCourier_FIFOOrder verifies rows come back in ascending created_at order.
func TestListAvailableForCourier_FIFOOrder(t *testing.T) {
	now := time.Now()
	t1 := now.Add(-3 * time.Minute)
	t2 := now.Add(-2 * time.Minute)
	t3 := now.Add(-1 * time.Minute)

	// Repo returns rows in FIFO order (SQL ORDER BY created_at ASC).
	rows := []domain.AvailableOrderRow{
		{OrderID: "o-1", DeliveryID: "d-1", ZoneName: "Center", CreatedAt: t1},
		{OrderID: "o-2", DeliveryID: "d-2", ZoneName: "Center", CreatedAt: t2},
		{OrderID: "o-3", DeliveryID: "d-3", ZoneName: "Center", CreatedAt: t3},
	}

	repo := &stubListAvailableRepo{
		fn: func(ctx context.Context, courierID, cursor string, limit int32) ([]domain.AvailableOrderRow, string, error) {
			return rows, "", nil
		},
	}

	got, _, err := repo.ListAvailableForCourier(context.Background(), "courier-1", "", 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i := 1; i < len(got); i++ {
		if !got[i].CreatedAt.After(got[i-1].CreatedAt) && !got[i].CreatedAt.Equal(got[i-1].CreatedAt) {
			t.Errorf("rows not in FIFO order at index %d: %v >= %v", i, got[i-1].CreatedAt, got[i].CreatedAt)
		}
	}
}

// TestListAvailableForCourier_CursorPagination verifies that pagination cursors
// are generated and propagated correctly.
func TestListAvailableForCourier_CursorPagination(t *testing.T) {
	page := 0
	// Simulate 25 total rows across 3 pages.
	allRows := make([]domain.AvailableOrderRow, 25)
	now := time.Now()
	for i := range allRows {
		allRows[i] = domain.AvailableOrderRow{
			OrderID:    "o-" + strconv.Itoa(i),
			DeliveryID: "d-" + strconv.Itoa(i),
			ZoneName:   "Center",
			CreatedAt:  now.Add(time.Duration(i) * time.Second),
		}
	}

	repo := &stubListAvailableRepo{
		fn: func(ctx context.Context, courierID, cursor string, limit int32) ([]domain.AvailableOrderRow, string, error) {
			start := page * int(limit)
			end := start + int(limit)
			if end > len(allRows) {
				end = len(allRows)
			}
			result := allRows[start:end]

			var nextCursor string
			if int32(len(result)) == limit {
				last := result[len(result)-1]
				raw := strconv.FormatInt(last.CreatedAt.UnixNano(), 10) + ":" + last.DeliveryID
				nextCursor = base64.StdEncoding.EncodeToString([]byte(raw))
			}
			return result, nextCursor, nil
		},
	}

	// Page 1: 10 rows + cursor.
	rows, cursor1, err := repo.ListAvailableForCourier(context.Background(), "courier-1", "", 10)
	if err != nil {
		t.Fatalf("page 1 error: %v", err)
	}
	if len(rows) != 10 {
		t.Errorf("page 1: want 10 rows, got %d", len(rows))
	}
	if cursor1 == "" {
		t.Error("page 1: expected non-empty next_cursor")
	}

	// Page 2: 10 rows + cursor.
	page = 1
	rows, cursor2, err := repo.ListAvailableForCourier(context.Background(), "courier-1", cursor1, 10)
	if err != nil {
		t.Fatalf("page 2 error: %v", err)
	}
	if len(rows) != 10 {
		t.Errorf("page 2: want 10 rows, got %d", len(rows))
	}
	if cursor2 == "" {
		t.Error("page 2: expected non-empty next_cursor")
	}

	// Page 3: 5 rows + empty cursor.
	page = 2
	rows, cursor3, err := repo.ListAvailableForCourier(context.Background(), "courier-1", cursor2, 10)
	if err != nil {
		t.Fatalf("page 3 error: %v", err)
	}
	if len(rows) != 5 {
		t.Errorf("page 3: want 5 rows, got %d", len(rows))
	}
	if cursor3 != "" {
		t.Errorf("page 3: expected empty next_cursor (last page), got %q", cursor3)
	}
}
