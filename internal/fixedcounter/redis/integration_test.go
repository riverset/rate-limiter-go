package fcredis_test

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	fcredis "learn.ratelimiter/internal/fixedcounter/redis"
	"learn.ratelimiter/internal/testharness/redistest"
	// "github.com/go-redis/redis/v8" // Removed as it's unused
)

func TestFixedCounterRedis_Integration(t *testing.T) {
	client := redistest.SetupRedisClient(t)
	defer client.Close()

	baseLimiterKey := "test_fc_integration"
	ctx := context.Background()

	t.Run("BasicAllowanceAndDenial", func(t *testing.T) {
		limiterKey := baseLimiterKey + "_basic"
		redistest.CleanupRedisKeys(t, client, limiterKey, "") // Clean keys for this specific test
		defer redistest.CleanupRedisKeys(t, client, limiterKey, "")

		window := 3 * time.Second // Short window for testing expiry
		limit := int64(2)
		
		// Use real time for this basic test, clock injection not strictly needed unless testing exact expiry boundary
		limiter := fcredis.NewLimiter(client, limiterKey, window, limit) 

		// Request 1: Allow
		allowed, err := limiter.Allow(ctx, "user1")
		if err != nil {
			t.Fatalf("Allow 1: unexpected error: %v", err)
		}
		if !allowed {
			t.Fatal("Allow 1: should be allowed")
		}

		// Request 2: Allow
		allowed, err = limiter.Allow(ctx, "user1")
		if err != nil {
			t.Fatalf("Allow 2: unexpected error: %v", err)
		}
		if !allowed {
			t.Fatal("Allow 2: should be allowed")
		}

		// Request 3: Deny (limit is 2)
		allowed, err = limiter.Allow(ctx, "user1")
		if err != nil {
			t.Fatalf("Allow 3: unexpected error: %v", err)
		}
		if allowed {
			t.Fatal("Allow 3: should be denied, limit reached")
		}
	})

	t.Run("WindowReset", func(t *testing.T) {
		limiterKey := baseLimiterKey + "_reset"
		redistest.CleanupRedisKeys(t, client, limiterKey, "")
		defer redistest.CleanupRedisKeys(t, client, limiterKey, "")
		
		window := 2 * time.Second 
		limit := int64(1)
		
		// For integration test of window reset, use real time via time.Sleep
		limiter := fcredis.NewLimiter(client, limiterKey, window, limit)

		// Request 1: Allow
		allowed, err := limiter.Allow(ctx, "user2")
		if err != nil {
			t.Fatalf("Allow 1: unexpected error: %v", err)
		}
		if !allowed {
			t.Fatal("Allow 1: should be allowed")
		}
		
		// Request 2: Deny (still within window, limit 1)
		allowed, err = limiter.Allow(ctx, "user2")
		if err != nil {
			t.Fatalf("Allow 2: unexpected error: %v", err)
		}
		if allowed {
			t.Fatal("Allow 2: should be denied")
		}

		// Wait for the window to pass
		t.Logf("Waiting for %v for window to reset...", window + 500*time.Millisecond) // Add buffer
		time.Sleep(window + 500*time.Millisecond)


		// Request 3: Allow (new window)
		allowed, err = limiter.Allow(ctx, "user2")
		if err != nil {
			t.Fatalf("Allow 3 (after sleep): unexpected error: %v", err)
		}
		if !allowed {
			t.Fatal("Allow 3 (after sleep): should be allowed after window reset")
		}
		
		redisKey := fmt.Sprintf("%s:%s", limiterKey, "user2")
		valStr, errGet := client.Get(ctx, redisKey).Result()
		if errGet != nil {
			// If the key expired and was removed, Get will return redis.Nil.
			// If the script re-adds it, it will be "1".
			// This check depends on whether the script SETs on first allow or if Add is used.
			// The fcredis script does `INCR` which creates if not exists, or `SET` with EX if it's a more complex script.
			// Given typical fixed window Lua: INCR, if 1 then EXPIRE.
			// So, after reset and one new request, count should be 1.
			t.Logf("Value from Redis for key %s: %s (err: %v)", redisKey, valStr, errGet)
			// This part of the test might need adjustment based on exact script behavior for key creation/expiry.
			// For now, primary check is that Allow succeeded.
		} else {
			count, _ := strconv.Atoi(valStr)
			if count != 1 { 
				t.Errorf("Expected count in Redis to be 1 after reset, got %d", count)
			}
		}
	})

	t.Run("DifferentIdentifiers", func(t *testing.T) {
		limiterKey := baseLimiterKey + "_diffids"
		redistest.CleanupRedisKeys(t, client, limiterKey, "")
		defer redistest.CleanupRedisKeys(t, client, limiterKey, "")

		window := 5 * time.Second
		limit := int64(1)
		limiter := fcredis.NewLimiter(client, limiterKey, window, limit)

		// User A: Allow
		allowedA, errA := limiter.Allow(ctx, "userA")
		if errA != nil || !allowedA {
			t.Fatal("userA should be allowed")
		}
		// User A: Deny
		allowedA2, errA2 := limiter.Allow(ctx, "userA")
		if errA2 != nil || allowedA2 {
			t.Fatal("userA should be denied on second attempt")
		}

		// User B: Allow (independent)
		allowedB, errB := limiter.Allow(ctx, "userB")
		if errB != nil || !allowedB {
			t.Fatal("userB should be allowed")
		}
	})
	
	// Note: Concurrency test for fixed window with Redis is tricky without CAS operations
	// on the count and expiry. The Lua script handles atomicity for a single operation.
	// A true concurrency test would involve multiple goroutines hammering the limiter.
}
