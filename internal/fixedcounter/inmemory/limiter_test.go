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
