package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	t.Cleanup(mr.Close)

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })
	return mr, rdb
}

func newTestHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func doRequest(t *testing.T, handler http.Handler, ip, userID string) int {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = ip + ":12345"
	if userID != "" {
		ctx := context.WithValue(req.Context(), ctxUserID, userID)
		req = req.WithContext(ctx)
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr.Code
}

func TestRateLimit_IPUnderLimit(t *testing.T) {
	_, rdb := newTestRedis(t)
	h := RateLimit(rdb, 100, 500)(newTestHandler())

	for i := 0; i < 99; i++ {
		if code := doRequest(t, h, "192.168.1.1", ""); code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, code)
		}
	}
}

func TestRateLimit_IPOverLimit(t *testing.T) {
	_, rdb := newTestRedis(t)
	h := RateLimit(rdb, 100, 500)(newTestHandler())

	// Make 100 requests (at limit)
	for i := 0; i < 100; i++ {
		doRequest(t, h, "10.0.0.1", "")
	}
	// 101st should be rate limited
	if code := doRequest(t, h, "10.0.0.1", ""); code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", code)
	}
}

func TestRateLimit_UserUnderLimit(t *testing.T) {
	_, rdb := newTestRedis(t)
	h := RateLimit(rdb, 10000, 500)(newTestHandler()) // high IP limit, 500 user limit

	for i := 0; i < 499; i++ {
		if code := doRequest(t, h, "10.1.1.1", "user-abc"); code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, code)
		}
	}
}

func TestRateLimit_UserOverLimit(t *testing.T) {
	_, rdb := newTestRedis(t)
	h := RateLimit(rdb, 10000, 500)(newTestHandler())

	// Make 500 user requests
	for i := 0; i < 500; i++ {
		doRequest(t, h, "10.2.2.2", "user-xyz")
	}
	// 501st should be rate limited
	if code := doRequest(t, h, "10.2.2.2", "user-xyz"); code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", code)
	}
}

func TestRateLimit_DifferentIPs(t *testing.T) {
	_, rdb := newTestRedis(t)
	h := RateLimit(rdb, 100, 500)(newTestHandler())

	// Each IP gets its own independent counter
	for i := 0; i < 99; i++ {
		if code := doRequest(t, h, "192.168.0.1", ""); code != http.StatusOK {
			t.Fatalf("IP1 request %d: expected 200, got %d", i+1, code)
		}
		if code := doRequest(t, h, "192.168.0.2", ""); code != http.StatusOK {
			t.Fatalf("IP2 request %d: expected 200, got %d", i+1, code)
		}
	}
}

func TestRateLimit_ExpiresAfterMinute(t *testing.T) {
	mr, rdb := newTestRedis(t)
	h := RateLimit(rdb, 100, 500)(newTestHandler())

	// Fill up to limit
	for i := 0; i < 100; i++ {
		doRequest(t, h, "172.16.0.1", "")
	}
	// Should be at limit now
	if code := doRequest(t, h, "172.16.0.1", ""); code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 before expiry, got %d", code)
	}

	// Fast-forward time by 1 minute to expire the counter
	mr.FastForward(time.Minute)

	// Counter should be reset now
	if code := doRequest(t, h, "172.16.0.1", ""); code != http.StatusOK {
		t.Fatalf("expected 200 after expiry, got %d", code)
	}
}
