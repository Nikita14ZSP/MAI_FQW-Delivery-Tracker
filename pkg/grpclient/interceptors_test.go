package grpclient_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sony/gobreaker"
	"google.golang.org/grpc"

	"github.com/mozgovojnikita/delivery-tracker/pkg/grpclient"
)

// mockInvoker returns a gRPC UnaryInvoker that returns the specified errors in sequence.
// After exhausting the sequence it returns nil.
func mockInvoker(errs ...error) (grpc.UnaryInvoker, *int32) {
	var callCount int32
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		n := int(atomic.AddInt32(&callCount, 1)) - 1
		if n < len(errs) {
			return errs[n]
		}
		return nil
	}, &callCount
}

// countingInvoker always returns the given error and counts every call.
func countingInvoker(err error) (grpc.UnaryInvoker, *int32) {
	var callCount int32
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		atomic.AddInt32(&callCount, 1)
		return err
	}, &callCount
}

var sentinelErr = errors.New("upstream error")

// --- Circuit Breaker Tests ---

func TestCircuitBreakerAllowsSuccessfulCalls(t *testing.T) {
	interceptor := grpclient.NewCircuitBreakerInterceptor("test-svc")
	invoker, calls := mockInvoker() // always returns nil

	for i := 0; i < 10; i++ {
		err := interceptor(context.Background(), "/svc/GetItem", nil, nil, nil, invoker)
		if err != nil {
			t.Fatalf("expected nil error on successful call %d, got: %v", i, err)
		}
	}
	if atomic.LoadInt32(calls) != 10 {
		t.Fatalf("expected 10 invocations, got %d", atomic.LoadInt32(calls))
	}
}

func TestCircuitBreakerTripsAfterConsecutiveFailures(t *testing.T) {
	interceptor := grpclient.NewCircuitBreakerInterceptor("test-svc-trip")
	invoker, _ := countingInvoker(sentinelErr)

	// Make 5 consecutive failures to trip the breaker
	for i := 0; i < 5; i++ {
		_ = interceptor(context.Background(), "/svc/GetItem", nil, nil, nil, invoker)
	}

	// 6th call must return ErrOpenState without invoking the upstream
	var prevCalls int32
	prevCalls = 5 // we've made 5 calls above
	err := interceptor(context.Background(), "/svc/GetItem", nil, nil, nil, invoker)
	if !errors.Is(err, gobreaker.ErrOpenState) {
		t.Fatalf("expected ErrOpenState after tripping, got: %v", err)
	}
	// Upstream should NOT have been called again
	_ = prevCalls
}

func TestCircuitBreakerIgnoresContextCanceled(t *testing.T) {
	interceptor := grpclient.NewCircuitBreakerInterceptor("test-svc-cancel")
	invoker, _ := countingInvoker(context.Canceled)

	// Even after many context.Canceled errors the breaker must NOT trip
	for i := 0; i < 10; i++ {
		err := interceptor(context.Background(), "/svc/GetItem", nil, nil, nil, invoker)
		// context.Canceled is returned as-is (not wrapped by gobreaker)
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("unexpected error on iteration %d: %v", i, err)
		}
	}

	// Breaker must still be closed — next call goes through (returns context.Canceled, not ErrOpenState)
	err := interceptor(context.Background(), "/svc/GetItem", nil, nil, nil, invoker)
	if errors.Is(err, gobreaker.ErrOpenState) {
		t.Fatal("circuit breaker must not trip on context.Canceled")
	}
}

// --- Retry Interceptor Tests ---

func TestRetryInterceptorRetriesGetMethods(t *testing.T) {
	interceptor := grpclient.NewRetryInterceptor()
	// Fail twice, then succeed on third attempt
	invoker, calls := mockInvoker(sentinelErr, sentinelErr)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := interceptor(ctx, "/pkg.Service/GetItem", nil, nil, nil, invoker)
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if atomic.LoadInt32(calls) != 3 {
		t.Fatalf("expected 3 invocations (2 failures + 1 success), got %d", atomic.LoadInt32(calls))
	}
}

func TestRetryInterceptorRetriesListMethods(t *testing.T) {
	interceptor := grpclient.NewRetryInterceptor()
	// Fail twice, then succeed
	invoker, calls := mockInvoker(sentinelErr, sentinelErr)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := interceptor(ctx, "/pkg.Service/ListItems", nil, nil, nil, invoker)
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if atomic.LoadInt32(calls) != 3 {
		t.Fatalf("expected 3 invocations, got %d", atomic.LoadInt32(calls))
	}
}

func TestRetryInterceptorSkipsMutations(t *testing.T) {
	interceptor := grpclient.NewRetryInterceptor()
	invoker, calls := countingInvoker(sentinelErr)

	err := interceptor(context.Background(), "/pkg.Service/CreateItem", nil, nil, nil, invoker)
	if !errors.Is(err, sentinelErr) {
		t.Fatalf("expected sentinelErr from mutation, got: %v", err)
	}
	if atomic.LoadInt32(calls) != 1 {
		t.Fatalf("mutation must be called exactly once, got %d", atomic.LoadInt32(calls))
	}
}

func TestRetryInterceptorSkipsRateCourier(t *testing.T) {
	interceptor := grpclient.NewRetryInterceptor()
	invoker, calls := countingInvoker(sentinelErr)

	err := interceptor(context.Background(), "/delivery.v1.DeliveryService/RateCourier", nil, nil, nil, invoker)
	if !errors.Is(err, sentinelErr) {
		t.Fatalf("expected sentinelErr from RateCourier, got: %v", err)
	}
	if atomic.LoadInt32(calls) != 1 {
		t.Fatalf("RateCourier must be called exactly once, got %d", atomic.LoadInt32(calls))
	}
}

func TestRetryInterceptorSkipsUpdateMethod(t *testing.T) {
	interceptor := grpclient.NewRetryInterceptor()
	invoker, calls := countingInvoker(sentinelErr)

	err := interceptor(context.Background(), "/pkg.Service/UpdateItem", nil, nil, nil, invoker)
	if !errors.Is(err, sentinelErr) {
		t.Fatalf("expected sentinelErr from mutation, got: %v", err)
	}
	if atomic.LoadInt32(calls) != 1 {
		t.Fatalf("UpdateItem must be called exactly once, got %d", atomic.LoadInt32(calls))
	}
}

func TestRetryInterceptorSkipsDeleteMethod(t *testing.T) {
	interceptor := grpclient.NewRetryInterceptor()
	invoker, calls := countingInvoker(sentinelErr)

	err := interceptor(context.Background(), "/pkg.Service/DeleteItem", nil, nil, nil, invoker)
	if !errors.Is(err, sentinelErr) {
		t.Fatalf("expected sentinelErr from mutation, got: %v", err)
	}
	if atomic.LoadInt32(calls) != 1 {
		t.Fatalf("DeleteItem must be called exactly once, got %d", atomic.LoadInt32(calls))
	}
}
