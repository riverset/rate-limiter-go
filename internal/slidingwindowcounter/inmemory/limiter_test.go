// Package inmemory_test contains tests for the in-memory sliding window counter.
package inmemory_test

import (
	"context"
	"testing"
	"time"

	swcinmemory "learn.ratelimiter/internal/slidingwindowcounter/inmemory"
)

func TestSlidingWindowLimiter(t *testing.T) {
	limiter := swcinmemory.NewLimiter("test_sliding_window", 100*time.Millisecond, 3)
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

	// Test sliding window behavior (this is a basic test, more comprehensive tests needed)
	time.Sleep(50 * time.Millisecond) // Move halfway into the window

	// Allow one more request, the oldest one should be just outside the window
	allowed, err = limiter.Allow(ctx, "user1")
	if err != nil {
		t.Fatalf("Allow failed: %v", err)
	}
	if !allowed {
		t.Fatalf("Request unexpectedly denied in sliding window")
	}

	// Deny the next request
	allowed, err = limiter.Allow(ctx, "user1")
	if err != nil {
		t.Fatalf("Allow failed: %v", err)
	}
	if allowed {
		t.Fatalf("Request unexpectedly allowed in sliding window after limit")
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
