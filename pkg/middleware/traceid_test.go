package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mozgovojnikita/delivery-tracker/pkg/middleware"
)

func TestTraceID_GeneratesWhenMissing(t *testing.T) {
	handler := middleware.TraceID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	traceID := rr.Header().Get("X-Trace-ID")
	if traceID == "" {
		t.Error("expected X-Trace-ID header to be set when not provided in request")
	}
	// UUID format check: should be non-empty and of reasonable length
	if len(traceID) < 10 {
		t.Errorf("expected UUID-like trace ID, got '%s'", traceID)
	}
}

func TestTraceID_PreservesExisting(t *testing.T) {
	handler := middleware.TraceID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Trace-ID", "test-123")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	traceID := rr.Header().Get("X-Trace-ID")
	if traceID != "test-123" {
		t.Errorf("expected X-Trace-ID 'test-123', got '%s'", traceID)
	}
}

func TestLoggerFromContext_Default(t *testing.T) {
	l := middleware.LoggerFromContext(context.Background())
	if l == nil {
		t.Error("expected non-nil logger from empty context")
	}
}
