// Internal test package: these tests exercise the unexported statusWriter
// type and its http.Hijacker / http.Flusher implementations directly, which
// is impossible from an external (_test) package.
package middleware

import (
	"bufio"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Compile-time guarantees: *statusWriter must satisfy the interfaces that
// httputil.ReverseProxy type-asserts for WebSocket/SSE protocol upgrades and
// streaming. If these fail, the api-gateway WS proxy returns HTTP 502.
var (
	_ http.Hijacker       = (*statusWriter)(nil)
	_ http.Flusher        = (*statusWriter)(nil)
	_ http.ResponseWriter = (*statusWriter)(nil)
)

// hijackableRecorder wraps httptest.ResponseRecorder and additionally
// implements http.Hijacker + http.Flusher so we can assert delegation.
type hijackableRecorder struct {
	*httptest.ResponseRecorder
	hijacked bool
	flushed  bool
}

func (h *hijackableRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h.hijacked = true
	// Return a sentinel error so the test can recognise that delegation
	// reached this underlying writer (no real net.Conn needed).
	return nil, nil, errSentinelHijack
}

func (h *hijackableRecorder) Flush() {
	h.flushed = true
}

var errSentinelHijack = errors.New("sentinel: underlying hijack reached")

// plainRecorder implements only http.ResponseWriter (no Hijacker/Flusher).
type plainRecorder struct {
	*httptest.ResponseRecorder
}

func TestStatusWriter_Hijack_Delegates(t *testing.T) {
	underlying := &hijackableRecorder{ResponseRecorder: httptest.NewRecorder()}
	sw := &statusWriter{ResponseWriter: underlying, code: http.StatusOK}

	_, _, err := sw.Hijack()

	if !underlying.hijacked {
		t.Fatal("expected Hijack() to delegate to the underlying http.Hijacker")
	}
	if !errors.Is(err, errSentinelHijack) {
		t.Fatalf("expected sentinel error from delegated Hijack, got %v", err)
	}
}

func TestStatusWriter_Hijack_NonHijackerReturnsError(t *testing.T) {
	underlying := &plainRecorder{ResponseRecorder: httptest.NewRecorder()}
	sw := &statusWriter{ResponseWriter: underlying, code: http.StatusOK}

	conn, rw, err := sw.Hijack()

	if err == nil {
		t.Fatal("expected non-nil error when underlying ResponseWriter is not an http.Hijacker")
	}
	if conn != nil || rw != nil {
		t.Fatalf("expected nil conn and bufrw on hijack failure, got conn=%v rw=%v", conn, rw)
	}
}

func TestStatusWriter_Flush_Delegates(t *testing.T) {
	underlying := &hijackableRecorder{ResponseRecorder: httptest.NewRecorder()}
	sw := &statusWriter{ResponseWriter: underlying, code: http.StatusOK}

	sw.Flush()

	if !underlying.flushed {
		t.Fatal("expected Flush() to delegate to the underlying http.Flusher")
	}
}

func TestStatusWriter_Flush_NoopWhenUnsupported(t *testing.T) {
	underlying := &plainRecorder{ResponseRecorder: httptest.NewRecorder()}
	sw := &statusWriter{ResponseWriter: underlying, code: http.StatusOK}

	// Must not panic when the underlying writer does not implement http.Flusher.
	sw.Flush()
}

func TestStatusWriter_WriteHeader_StillCapturesStatus(t *testing.T) {
	// Smoke test: the Metrics middleware must keep recording status via the
	// statusWriter wrapper after the Hijacker/Flusher additions.
	handler := Metrics(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/orders", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusTeapot {
		t.Fatalf("expected status %d to propagate through statusWriter, got %d", http.StatusTeapot, rr.Code)
	}
}
