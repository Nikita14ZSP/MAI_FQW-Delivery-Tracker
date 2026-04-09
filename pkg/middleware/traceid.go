package middleware

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
)

// ctxKey is a private type for context keys to avoid collisions.
type ctxKey string

const (
	// LoggerKey is the context key for storing a request-scoped logger.
	LoggerKey ctxKey = "logger"
	// TraceIDKey is the context key for storing the request trace ID.
	TraceIDKey ctxKey = "trace_id"
)

// TraceID is an HTTP middleware that injects a trace ID into every request.
// If the incoming request contains an X-Trace-ID header, that value is reused;
// otherwise a new UUID is generated. The trace ID is stored in the request
// context and returned in the X-Trace-ID response header.
func TraceID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceID := r.Header.Get("X-Trace-ID")
		if traceID == "" {
			traceID = uuid.NewString()
		}

		w.Header().Set("X-Trace-ID", traceID)

		logger := slog.Default().With("trace_id", traceID)

		ctx := context.WithValue(r.Context(), LoggerKey, logger)
		ctx = context.WithValue(ctx, TraceIDKey, traceID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// LoggerFromContext retrieves the request-scoped logger from the context.
// If no logger is found, slog.Default() is returned.
func LoggerFromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(LoggerKey).(*slog.Logger); ok && l != nil {
		return l
	}
	return slog.Default()
}
