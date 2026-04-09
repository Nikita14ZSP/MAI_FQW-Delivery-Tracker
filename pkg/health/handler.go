package health

import (
	"net/http"
)

// Handler returns an HTTP handler function that responds with a JSON health status.
// It always returns HTTP 200 with body {"status":"ok"}.
func Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`)) //nolint:errcheck
	}
}
