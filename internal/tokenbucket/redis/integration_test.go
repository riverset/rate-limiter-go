// Package redistb_test contains integration tests for the Redis Token Bucket rate limiting algorithm.
package tbredis_test

import (
	"context"
	"fmt"
	"testing"
	"time"
	// "os" // No longer needed directly here

	// "github.com/go-redis/redis/v8" // No longer needed directly here
	"learn.ratelimiter/internal/testharness/redistest" // Import shared helper
	redistb "learn.ratelimiter/internal/tokenbucket/redis"
)

// const tokenBucketPatternPrefix = "token_bucket" // This constant is not strictly necessary if keys are managed per test.

func TestRedisTokenBucketLimiter(t *testing.T) {
	client := redistest.SetupRedisClient(t) 
	defer client.Close()

	baseLimiterKey := "test_redis_tb_limiter" // Use a base key for this test suite
	ctx := context.Background()

	// Test case 1: Basic rate limiting within capacity
	limiter1Key := baseLimiterKey + "_basic"
	redistest.CleanupRedisKeys(t, client, limiter1Key, "") 
	limiter1 := redistb.NewLimiter(limiter1Key, 10, 50, client)

	for i := 0; i < 50; i++ {
		allowed, err := limiter1.Allow(ctx, "user1")
		if err != nil {
			t.Fatalf("TestBasic: Allow failed on request %d: %v", i+1, err)
		}
		if !allowed {
			t.Fatalf("TestBasic: Request %d unexpectedly denied", i+1)
		}
	}

	// Test case 2: Deny requests over capacity
	allowed, err := limiter1.Allow(ctx, "user1") // This is still user1 for limiter1Key
	if err != nil {
		t.Fatalf("TestBasic: Allow failed on request after capacity: %v", err)
	}
	if allowed {
		t.Fatalf("TestBasic: Request unexpectedly allowed after capacity")
	}

	// Test case 3: Token refill over time
	limiter2Key := baseLimiterKey + "_refill" // Use baseLimiterKey
	redistest.CleanupRedisKeys(t, client, limiter2Key, "") // Clean for this sub-test
	limiter2 := redistb.NewLimiter(limiter2Key, 10, 10, client) // Rate 10/sec, Capacity 10

	// Consume all initial tokens
	for i := 0; i < 10; i++ {
		allowed, err := limiter2.Allow(ctx, "user2")
		if err != nil {
			t.Fatalf("TestRefill: Allow failed on initial request %d: %v", i+1, err)
		}
		if !allowed {
			t.Fatalf("TestRefill: Initial request %d unexpectedly denied", i+1)
		}
	}

	// Wait for tokens to refill (at least 1 second for 10 tokens)
	time.Sleep(1 * time.Second)

	// Should allow requests again
	allowed, err = limiter2.Allow(ctx, "user2")
	if err != nil {
		t.Fatalf("TestRefill: Allow failed after refill: %v", err)
	}
	if !allowed {
		t.Fatalf("TestRefill: Request unexpectedly denied after refill")
	}
}

func TestRedisTokenBucketConcurrencyAndEdgeCases(t *testing.T) {
	client := redistest.SetupRedisClient(t) // Use shared helper
	defer client.Close()

	limiterKeyBase := "test_redis_token_bucket_concurrency" 
	ctx := context.Background() // Define ctx

	// Test case 1: Concurrency
	// Note: Concurrency tests on a shared Redis can be flaky if keys are not isolated.
	// Using a unique key for the concurrency part of the test.
	concurrencyLimiterKey := limiterKeyBase + "_concurrency_run" 
	redistest.CleanupRedisKeys(t, client, concurrencyLimiterKey, "")
	limiter := redistb.NewLimiter(concurrencyLimiterKey, 10, 50, client) 
	
	t.Run("Concurrency", func(t *testing.T) {
		concurrentRequests := 100
		allowedCount := 0
		errors := make(chan error, concurrentRequests)
		results := make(chan bool, concurrentRequests)

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

		if allowedCount > 50 {
			t.Fatalf("Concurrency test failed: allowed more requests (%d) than capacity (50)", allowedCount)
		}
	})

	// Test case 2: Edge cases
	t.Run("EdgeCases", func(t *testing.T) {
		// High rate and capacity
		edgeLimiterKey := limiterKeyBase + "_high"
		redistest.CleanupRedisKeys(t, client, edgeLimiterKey, "")
		limiterHigh := redistb.NewLimiter(edgeLimiterKey, 1000, 10000, client)
		for i := 0; i < 10000; i++ { // This will create 10000 different identifiers
			allowed, err := limiterHigh.Allow(ctx, fmt.Sprintf("user_high_%d", i))
			if err != nil {
				t.Fatalf("Edge case test failed: %v", err)
			}
			if !allowed {
				t.Fatalf("Edge case test failed: request unexpectedly denied for user_high_%d", i)
			}
		}
	})
}
