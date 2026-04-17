package middleware

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	httpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests",
	}, []string{"method", "path", "status"})

	httpRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request duration in seconds",
		Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
	}, []string{"method", "path"})

	httpRequestsInFlight = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "http_requests_in_flight",
		Help: "Number of HTTP requests currently being processed",
	})
)

type statusWriter struct {
	http.ResponseWriter
	code int
}

func (w *statusWriter) WriteHeader(code int) {
	w.code = code
	w.ResponseWriter.WriteHeader(code)
}

// Hijack lets the WebSocket/SSE protocol switch work through this wrapper
// (httputil.ReverseProxy type-asserts http.Hijacker for protocol upgrades).
func (w *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("metrics: underlying ResponseWriter does not implement http.Hijacker")
	}
	return h.Hijack()
}

// Flush delegates to the underlying ResponseWriter when it supports flushing
// (streaming / SSE / proxied responses).
func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Metrics is an HTTP middleware that records request count, duration and in-flight gauge.
func Metrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, code: http.StatusOK}

		httpRequestsInFlight.Inc()
		defer httpRequestsInFlight.Dec()

		next.ServeHTTP(sw, r)

		duration := time.Since(start).Seconds()
		path := normalizePath(r.URL.Path)
		status := strconv.Itoa(sw.code)

		httpRequestsTotal.WithLabelValues(r.Method, path, status).Inc()
		httpRequestDuration.WithLabelValues(r.Method, path).Observe(duration)
	})
}

// normalizePath reduces cardinality by replacing UUIDs and IDs with placeholders.
func normalizePath(p string) string {
	// Keep first two segments: /v1/orders/{id}/status → /v1/orders/:id/status
	parts := []byte(p)
	result := make([]byte, 0, len(parts))
	segStart := 0
	segIdx := 0
	for i := 0; i <= len(parts); i++ {
		if i == len(parts) || parts[i] == '/' {
			if segIdx > 0 {
				seg := string(parts[segStart:i])
				if looksLikeID(seg) {
					result = append(result, ":id"...)
				} else {
					result = append(result, seg...)
				}
			}
			if i < len(parts) {
				result = append(result, '/')
			}
			segStart = i + 1
			segIdx++
		}
	}
	if len(result) == 0 {
		return p
	}
	return string(result)
}

func looksLikeID(s string) bool {
	if len(s) < 8 {
		return false
	}
	hexChars := 0
	for _, c := range s {
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || c == '-' {
			hexChars++
		}
	}
	return hexChars == len(s)
}
