package logger

import (
	"io"
	"log/slog"
	"os"
)

// New creates a structured logger based on the environment.
// In "production" mode it uses JSON output; otherwise text output is used.
func New(env string) *slog.Logger {
	return NewWithWriter(env, os.Stdout)
}

// NewWithService creates a logger pre-tagged with a service name.
func NewWithService(env, serviceName string) *slog.Logger {
	return New(env).With("service", serviceName)
}

// NewWithWriter creates a logger writing to the provided writer.
// This is useful for testing. In "production" mode JSON handler is used;
// otherwise text handler is used.
func NewWithWriter(env string, w io.Writer) *slog.Logger {
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}
	var handler slog.Handler
	if env == "production" {
		handler = slog.NewJSONHandler(w, opts)
	} else {
		handler = slog.NewTextHandler(w, opts)
	}
	return slog.New(handler)
}

// NewWithServiceWriter creates a logger with a service name tag writing to a custom writer.
func NewWithServiceWriter(env, serviceName string, w io.Writer) *slog.Logger {
	return NewWithWriter(env, w).With("service", serviceName)
}
