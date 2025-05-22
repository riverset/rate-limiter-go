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

func TestLeakyBucketLimiter_ZeroRate(t *testing.T) {
	limiter := lbinmemory.NewLimiter("test_zero_rate", 0, 10) // Rate: 0 tokens/sec, Capacity: 10
	ctx := context.Background()

	// Requests should be denied as rate is 0
	// First, fill the capacity. These should be allowed.
	capacity := 10
	for i := 0; i < capacity; i++ {
		allowed, err := limiter.Allow(ctx, "user1")
		if err != nil {
			t.Fatalf("Unexpected error during capacity fill: %v", err)
		}
		if !allowed {
			t.Fatalf("Request %d/%d unexpectedly denied during capacity fill with zero rate", i+1, capacity)
		}
	}

	// Next request should be denied as rate is 0 and capacity is full
	allowed, err := limiter.Allow(ctx, "user1")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if allowed {
		t.Fatalf("Request unexpectedly allowed with zero rate after capacity is full")
	}
}

func TestLeakyBucketLimiter_NegativeRate(t *testing.T) {
	limiter := lbinmemory.NewLimiter("test_negative_rate", -1, 10) // Rate: -1 tokens/sec, Capacity: 10
	ctx := context.Background()
	capacity := 10

	// First, fill the capacity. These should be allowed.
	for i := 0; i < capacity; i++ {
		allowed, err := limiter.Allow(ctx, "user1")
		if err != nil {
			t.Fatalf("Unexpected error during capacity fill: %v", err)
		}
		if !allowed {
			t.Fatalf("Request %d/%d unexpectedly denied during capacity fill with negative rate", i+1, capacity)
		}
	}

	// Next request should be denied as rate is negative and capacity is full
	allowed, err := limiter.Allow(ctx, "user1")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if allowed {
		t.Fatalf("Request unexpectedly allowed with negative rate after capacity is full")
	}
}

func TestLeakyBucketLimiter_ZeroCapacity(t *testing.T) {
	limiter := lbinmemory.NewLimiter("test_zero_capacity", 5, 0) // Rate: 5 tokens/sec, Capacity: 0
	ctx := context.Background()

	// Requests should be denied as capacity is 0
	allowed, err := limiter.Allow(ctx, "user1")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if allowed {
		t.Fatalf("Request unexpectedly allowed with zero capacity")
	}
}

func TestLeakyBucketLimiter_NegativeCapacity(t *testing.T) {
	limiter := lbinmemory.NewLimiter("test_negative_capacity", 5, -1) // Rate: 5 tokens/sec, Capacity: -1
	ctx := context.Background()

	// Requests should be denied as capacity is negative
	allowed, err := limiter.Allow(ctx, "user1")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if allowed {
		t.Fatalf("Request unexpectedly allowed with negative capacity")
	}
}

func TestLeakyBucketLimiter_ContextCancellation(t *testing.T) {
	limiter := lbinmemory.NewLimiter("test_context_cancellation", 1, 1) // Rate: 1 token/sec, Capacity: 1
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
