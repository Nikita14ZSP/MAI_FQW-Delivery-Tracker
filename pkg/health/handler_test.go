package health_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mozgovojnikita/delivery-tracker/pkg/health"
)

func TestHandler_Returns200(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	health.Handler()(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestHandler_ReturnsJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	health.Handler()(rr, req)

	contentType := rr.Header().Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		t.Errorf("expected Content-Type 'application/json', got '%s'", contentType)
	}

	body := strings.TrimSpace(rr.Body.String())
	expected := `{"status":"ok"}`
	if body != expected {
		t.Errorf("expected body '%s', got '%s'", expected, body)
	}
}
