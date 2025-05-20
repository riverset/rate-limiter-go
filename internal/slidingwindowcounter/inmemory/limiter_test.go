// Package swinmemory_test contains tests for the in-memory sliding window counter.
package swinmemory_test

import (
	"context"
	"testing"
	"time"

	swinmemory "learn.ratelimiter/internal/slidingwindowcounter/inmemory"
)

func TestSlidingWindowLimiter(t *testing.T) {
	t.Run("SlidingWindowBehavior", func(t *testing.T) {
		limiter := swinmemory.NewLimiter("test_sliding_window_behavior", 100*time.Millisecond, 3)
		ctx := context.Background()

		// Fill the window
		for i := 0; i < 3; i++ {
			allowed, err := limiter.Allow(ctx, "user_sliding")
			if err != nil {
				t.Fatalf("Allow failed: %v", err)
			}
			if !allowed {
				t.Fatalf("Request %d unexpectedly denied", i+1)
			}
		}

		// Attempt one more request, should be denied
		allowed, err := limiter.Allow(ctx, "user_sliding")
		if err != nil {
			t.Fatalf("Allow failed: %v", err)
		}
		if allowed {
			t.Fatalf("Request unexpectedly allowed after limit")
		}

		time.Sleep(120 * time.Millisecond) // Move more than 1 into the window

		// After 120ms, the weighted count should be 2.4.
		// Allowing one more request would make it 3.4, exceeding the limit of 3.
		// Both subsequent requests should be denied.

		// Attempt one more request, should be denied
		allowed, err = limiter.Allow(ctx, "user_sliding")
		if err != nil {
			t.Fatalf("Allow failed: %v", err)
		}
		if allowed {
			t.Fatalf("Request unexpectedly allowed after 120ms sleep")
		}

		// Attempt another request, should also be denied
		allowed, err = limiter.Allow(ctx, "user_sliding")
		if err != nil {
			t.Fatalf("Allow failed: %v", err)
		}
		if allowed {
			t.Fatalf("Request unexpectedly allowed after previous denial")
		}
	})

	t.Run("DifferentIdentifier", func(t *testing.T) {
		limiter := swinmemory.NewLimiter("test_sliding_window_different", 100*time.Millisecond, 3)
		ctx := context.Background()

		allowed, err := limiter.Allow(ctx, "user2")
		if err != nil {
			t.Fatalf("Allow failed: %v", err)
		}
		if !allowed {
			t.Fatalf("Request for different identifier unexpectedly denied")
		}
	})
}
