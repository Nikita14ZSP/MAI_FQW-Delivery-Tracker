package grpclient

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/sony/gobreaker"
	"google.golang.org/grpc"
)

// NewCircuitBreakerInterceptor creates a gRPC unary client interceptor
// that wraps calls with a circuit breaker (per D-10, D-12).
// Settings: 5 consecutive failures to trip, 30s timeout to half-open, 5 probes in half-open.
// context.Canceled does NOT count as failure (Research Pitfall 4).
func NewCircuitBreakerInterceptor(serviceName string) grpc.UnaryClientInterceptor {
	cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        serviceName,
		MaxRequests: 5,
		Interval:    10 * time.Second,
		Timeout:     30 * time.Second,
		ReadyToTrip: func(c gobreaker.Counts) bool {
			return c.ConsecutiveFailures >= 5
		},
		IsSuccessful: func(err error) bool {
			return err == nil || errors.Is(err, context.Canceled)
		},
	})

	return func(ctx context.Context, method string, req, reply any,
		cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		_, err := cb.Execute(func() (any, error) {
			return nil, invoker(ctx, method, req, reply, cc, opts...)
		})
		return err
	}
}

// NewRetryInterceptor creates a gRPC unary client interceptor
// that retries idempotent calls (Get/List) with exponential backoff (per D-11).
// Mutations (Create/Update/Delete/Assign/Rate) are NEVER retried.
// Max 3 retries with exponential backoff + jitter.
func NewRetryInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any,
		cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if !isIdempotent(method) {
			return invoker(ctx, method, req, reply, cc, opts...)
		}
		bo := backoff.WithContext(backoff.WithMaxRetries(backoff.NewExponentialBackOff(), 3), ctx)
		return backoff.Retry(func() error {
			return invoker(ctx, method, req, reply, cc, opts...)
		}, bo)
	}
}

// isIdempotent checks if a gRPC method is safe to retry.
// Only methods starting with "Get" or "List" (after the last "/") are idempotent.
func isIdempotent(method string) bool {
	parts := strings.Split(method, "/")
	methodName := parts[len(parts)-1]
	return strings.HasPrefix(methodName, "Get") || strings.HasPrefix(methodName, "List")
}
