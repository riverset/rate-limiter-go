// Package fcinmemory_test contains tests for the in-memory fixed window counter.
package fcinmemory_test

import (
	"context"
	"testing"
	"time"

	fcinmemory "learn.ratelimiter/internal/fixedcounter/inmemory"
)

func TestFixedWindowLimiter(t *testing.T) {
	limiter := fcinmemory.NewLimiter("test_fixed_window", 100*time.Millisecond, 3)
	ctx := context.Background()

	// Test allowing requests within the limit
	for i := 0; i < 3; i++ {
		allowed, err := limiter.Allow(ctx, "user1")
		if err != nil {
			t.Fatalf("Allow failed: %v", err)
		}
		if !allowed {
			t.Fatalf("Request %d unexpectedly denied", i+1)
		}
	}

	// Test denying requests over the limit within the same window
	allowed, err := limiter.Allow(ctx, "user1")
	if err != nil {
		t.Fatalf("Allow failed: %v", err)
	}
	if allowed {
		t.Fatalf("Request unexpectedly allowed after limit")
	}

	// Test window reset after time passes
	time.Sleep(100 * time.Millisecond)

	allowed, err = limiter.Allow(ctx, "user1")
	if err != nil {
		t.Fatalf("Allow failed: %v", err)
	}
	if !allowed {
		t.Fatalf("Request unexpectedly denied after window reset")
	}

	// Test a different identifier
	allowed, err = limiter.Allow(ctx, "user2")
	if err != nil {
		t.Fatalf("Allow failed: %v", err)
	}
	if !allowed {
		t.Fatalf("Request for different identifier unexpectedly denied")
	}
}

func TestFixedWindowLimiter_ZeroLimit(t *testing.T) {
	limiter := fcinmemory.NewLimiter("test_zero_limit", 100*time.Millisecond, 0)
	ctx := context.Background()

	allowed, err := limiter.Allow(ctx, "user1")
	if err != nil {
		t.Fatalf("Allow failed: %v", err)
	}
	if allowed {
		t.Fatalf("Request unexpectedly allowed with zero limit")
	}
}

func TestFixedWindowLimiter_NegativeLimit(t *testing.T) {
	limiter := fcinmemory.NewLimiter("test_negative_limit", 100*time.Millisecond, -1)
	ctx := context.Background()

	allowed, err := limiter.Allow(ctx, "user1")
	if err != nil {
		t.Fatalf("Allow failed: %v", err)
	}
	if allowed {
		t.Fatalf("Request unexpectedly allowed with negative limit")
	}
}

func TestFixedWindowLimiter_Concurrency(t *testing.T) {
	limiter := fcinmemory.NewLimiter("test_concurrency", 100*time.Millisecond, 3)
	ctx := context.Background()
	numRequests := 10
	results := make(chan bool, numRequests)
	for i := 0; i < numRequests; i++ {
		go func() {
			allowed, err := limiter.Allow(ctx, "user1")
			if err != nil {
				t.Errorf("Allow failed: %v", err)
				results <- false
				return
			}
			results <- allowed
		}()
	}

	allowedCount := 0
	for i := 0; i < numRequests; i++ {
		if <-results {
			allowedCount++
		}
	}

	if allowedCount > 3 {
		t.Fatalf("Allowed %d requests, expected at most 3", allowedCount)
	}
}

func TestFixedWindowLimiter_ContextCancellation(t *testing.T) {
	limiter := fcinmemory.NewLimiter("test_context_cancellation", 100*time.Millisecond, 1)
	ctx, cancel := context.WithCancel(context.Background())

	// Allow one request
	_, err := limiter.Allow(ctx, "user1")
	if err != nil {
		t.Fatalf("Allow failed: %v", err)
	}

	// Cancel context before next request
	cancel()

	_, err = limiter.Allow(ctx, "user1")
	if err == nil {
		t.Fatalf("Expected error due to context cancellation, but got nil")
	}
	if err != context.Canceled {
		t.Fatalf("Expected context.Canceled error, but got %v", err)
	}
}
