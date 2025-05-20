package lbinmemory_test

import (
	"context"
	"testing"
	"time"

	lbinmemory "learn.ratelimiter/internal/leakybucket/inmemory"
)

func TestLeakyBucketLimiter(t *testing.T) {
	limiter := lbinmemory.NewLimiter("test_key", 5, 10) // Rate: 5 tokens/sec, Capacity: 10
	ctx := context.Background()

	// Test case 1: Allow requests within capacity
	t.Run("AllowRequestsWithinCapacity", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			allowed, err := limiter.Allow(ctx, "user1")
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if !allowed {
				t.Fatalf("Request %d unexpectedly denied", i+1)
			}
		}
	})

	// Test case 2: Deny requests exceeding capacity
	t.Run("DenyRequestsExceedingCapacity", func(t *testing.T) {
		allowed, err := limiter.Allow(ctx, "user1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if allowed {
			t.Fatalf("Request unexpectedly allowed when bucket is full")
		}
	})

	// Test case 3: Leak tokens over time
	t.Run("LeakTokensOverTime", func(t *testing.T) {
		time.Sleep(2 * time.Second) // Wait for tokens to leak
		allowed, err := limiter.Allow(ctx, "user1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !allowed {
			t.Fatalf("Request unexpectedly denied after tokens leaked")
		}
	})

	// Test case 4: Concurrency
	t.Run("Concurrency", func(t *testing.T) {
		concurrentRequests := 20
		results := make(chan bool, concurrentRequests)
		errors := make(chan error, concurrentRequests)

		for i := 0; i < concurrentRequests; i++ {
			go func() {
				allowed, err := limiter.Allow(ctx, "user_concurrent")
				if err != nil {
					errors <- err
					return
				}
				results <- allowed
			}()
		}

		allowedCount := 0
		for i := 0; i < concurrentRequests; i++ {
			select {
			case err := <-errors:
				t.Fatalf("Concurrency test failed: %v", err)
			case allowed := <-results:
				if allowed {
					allowedCount++
				}
			}
		}

		if allowedCount > 10 {
			t.Fatalf("Concurrency test failed: allowed more requests (%d) than capacity (10)", allowedCount)
		}
	})
}
