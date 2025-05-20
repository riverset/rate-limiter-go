// Package redistb_test contains integration tests for the Redis Token Bucket rate limiting algorithm.
package tbredis_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	redistb "learn.ratelimiter/internal/tokenbucket/redis"
)

// setupRedisClient initializes a Redis client for testing.
// It assumes a Redis instance is running on the default address.
func setupRedisClient(t *testing.T) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379", // Default Redis address
		DB:   0,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Ping the Redis server to ensure connectivity
	if _, err := client.Ping(ctx).Result(); err != nil {
		t.Fatalf("Failed to connect to Redis: %v", err)
	}

	return client
}

// cleanupRedis clears keys used by a specific limiter key from Redis.
func cleanupRedis(t *testing.T, client *redis.Client, limiterKey string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use SCAN and DEL to find and delete keys with the limiter key prefix
	iter := client.Scan(ctx, 0, fmt.Sprintf("token_bucket:%s:*", limiterKey), 0).Iterator()
	var keysToDelete []string
	for iter.Next(ctx) {
		keysToDelete = append(keysToDelete, iter.Val())
	}
	if err := iter.Err(); err != nil {
		t.Fatalf("Failed to scan keys for cleanup: %v", err)
	}

	if len(keysToDelete) > 0 {
		if _, err := client.Del(ctx, keysToDelete...).Result(); err != nil {
			t.Fatalf("Failed to delete keys during cleanup: %v", err)
		}
	}
}

func TestRedisTokenBucketLimiter(t *testing.T) {
	client := setupRedisClient(t)
	defer client.Close()

	limiterKey := "test_redis_token_bucket"
	cleanupRedis(t, client, limiterKey)

	// Test case 1: Basic rate limiting within capacity
	limiter1 := redistb.NewLimiter(limiterKey+"_basic", 10, 50, client)
	ctx := context.Background()

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
	allowed, err := limiter1.Allow(ctx, "user1")
	if err != nil {
		t.Fatalf("TestBasic: Allow failed on request after capacity: %v", err)
	}
	if allowed {
		t.Fatalf("TestBasic: Request unexpectedly allowed after capacity")
	}

	// Test case 3: Token refill over time
	limiter2Key := limiterKey + "_refill"
	cleanupRedis(t, client, limiter2Key)
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
	client := setupRedisClient(t)
	defer client.Close()

	limiterKey := "test_redis_token_bucket_concurrency"
	cleanupRedis(t, client, limiterKey)

	limiter := redistb.NewLimiter(limiterKey, 10, 50, client) // Rate 10/sec, Capacity 50
	ctx := context.Background()

	// Test case 1: Concurrency
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
		// Invalid user key
		_, err := limiter.Allow(ctx, "")
		if err == nil {
			t.Fatalf("Edge case test failed: expected error for empty user key")
		}

		// High rate and capacity
		limiterHigh := redistb.NewLimiter(limiterKey+"_high", 1000, 10000, client)
		for i := 0; i < 10000; i++ {
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
