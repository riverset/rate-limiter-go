package swredis_test

import (
	"context"
	"fmt"
	// "strconv" // Likely unused in integration test if not directly checking Redis values
	"testing"
	"time"

	// "github.com/go-redis/redis/v8" // Likely unused if errors handled via strings or errors.Is
	swredis "learn.ratelimiter/internal/slidingwindowcounter/redis"
	"learn.ratelimiter/internal/testharness/redistest"
)

// Shared mock time for controlling time in tests
var mockTimeSWC = time.Date(2024, time.January, 1, 12, 0, 0, 0, time.UTC)

func mockNowFuncSWC(t *testing.T) func() time.Time {
	currentTime := mockTimeSWC
	return func() time.Time {
		// t.Logf("SlidingWindowCounter mockNowFuncSWC called, returning: %v", currentTime)
		return currentTime
	}
}

func advanceMockTimeSWC(duration time.Duration) {
	mockTimeSWC = mockTimeSWC.Add(duration)
}

func resetMockTimeSWC() {
	mockTimeSWC = time.Date(2024, time.January, 1, 12, 0, 0, 0, time.UTC)
}

func TestSlidingWindowRedis_Integration(t *testing.T) {
	client := redistest.SetupRedisClient(t)
	defer client.Close()

	baseLimiterKey := "test_swc_integration"
	ctx := context.Background()
	
	// Sliding window keys are just `l.key + ":" + identifier`.
	// So, for cleanup, patternPrefix will be the limiterKey itself.

	t.Run("BasicAllowanceAndDenial", func(t *testing.T) {
		limiterKey := baseLimiterKey + "_basic"
		redistest.CleanupRedisKeys(t, client, limiterKey, "")
		defer redistest.CleanupRedisKeys(t, client, limiterKey, "")

		resetMockTimeSWC()
		window := 3 * time.Second 
		limit := int64(2)      
		// Use injected clock for predictable behavior
		limiter := swredis.NewLimiter(limiterKey, window, limit, client, swredis.WithClock(mockNowFuncSWC(t)))

		// Request 1: Allow
		allowed, err := limiter.Allow(ctx, "user1")
		if err != nil { t.Fatalf("Allow 1: unexpected error: %v", err) }
		if !allowed { t.Fatal("Allow 1: should be allowed") }

		// Request 2: Allow
		advanceMockTimeSWC(500 * time.Millisecond) // Ensure distinct timestamps if script relies on it
		allowed, err = limiter.Allow(ctx, "user1")
		if err != nil { t.Fatalf("Allow 2: unexpected error: %v", err) }
		if !allowed { t.Fatal("Allow 2: should be allowed") }

		// Request 3: Deny (limit is 2)
		advanceMockTimeSWC(500 * time.Millisecond)
		allowed, err = limiter.Allow(ctx, "user1")
		if err != nil { t.Fatalf("Allow 3: unexpected error: %v", err) }
		if allowed { t.Fatal("Allow 3: should be denied, limit reached") }
		
		resetMockTimeSWC()
	})

	t.Run("SlidingWindowBehavior_PartialSlide", func(t *testing.T) {
		limiterKey := baseLimiterKey + "_slide"
		redistest.CleanupRedisKeys(t, client, limiterKey, "")
		defer redistest.CleanupRedisKeys(t, client, limiterKey, "")

		resetMockTimeSWC()
		window := 5 * time.Second 
		limit := int64(3)      
		limiter := swredis.NewLimiter(limiterKey, window, limit, client, swredis.WithClock(mockNowFuncSWC(t)))

		// Time 0s: Req 1 (user2) - Allowed (Count: 1)
		allowed, _ := limiter.Allow(ctx, "user2") 
		if !allowed {t.Fatal("T0 Req1: Denied")}

		// Time 1s: Req 2 (user2) - Allowed (Count: 2)
		advanceMockTimeSWC(1 * time.Second)
		allowed, _ = limiter.Allow(ctx, "user2")
		if !allowed {t.Fatal("T1 Req2: Denied")}
		
		// Time 2s: Req 3 (user2) - Allowed (Count: 3)
		advanceMockTimeSWC(1 * time.Second)
		allowed, _ = limiter.Allow(ctx, "user2")
		if !allowed {t.Fatal("T2 Req3: Denied")}

		// Time 3s: Req 4 (user2) - Denied (Count: 3, Limit 3)
		advanceMockTimeSWC(1 * time.Second)
		allowed, _ = limiter.Allow(ctx, "user2")
		if allowed {t.Fatal("T3 Req4: Allowed, should be denied")}
		
		// Time 5.5s: Window is now [0.5s, 5.5s]. First request at 0s has expired.
		// Count should be 2 (requests at 1s, 2s). One slot available.
		advanceMockTimeSWC(2*time.Second + 500*time.Millisecond) // Total time from T0 = 5.5s
		
		// At T=5.5s, window is [0.5s, 5.5s].
		// Timestamps currently in Redis (hypothetically, before Allow call): 0s, 1s, 2s.
		// Allow call will:
		// 1. Prune: remove timestamps < 0.5s. Request at 0s is removed.
		//    Remaining timestamps: 1s, 2s. Count = 2.
		// 2. Check limit: 2 < 3, so allowed.
		// 3. Add current time: Add 5.5s.
		//    New timestamps: 1s, 2s, 5.5s. Count = 3.
		allowed, _ = limiter.Allow(ctx, "user2")
		if !allowed {
			redisKey := fmt.Sprintf("%s:%s", limiterKey, "user2")
			count, _ := client.ZCard(ctx, redisKey).Result()
			t.Fatalf("T5.5 Req5: Denied, but one slot should be free. Current count in Redis before this Allow: %d", count)
		}

		redisKey := fmt.Sprintf("%s:%s", limiterKey, "user2")
		finalCount, _ := client.ZCard(ctx, redisKey).Result()
		if finalCount != limit { // Should be 3 (timestamps: 1s, 2s, 5.5s)
			t.Errorf("Expected final count to be %d, got %d. Timestamps: %v", limit, finalCount, client.ZRange(ctx, redisKey, 0, -1).Val())
		}
		
		resetMockTimeSWC()
	})
	
	t.Run("WindowFullyReset", func(t *testing.T) {
		limiterKey := baseLimiterKey + "_fullreset"
		redistest.CleanupRedisKeys(t, client, limiterKey, "")
		defer redistest.CleanupRedisKeys(t, client, limiterKey, "")

		resetMockTimeSWC()
		window := 2 * time.Second 
		limit := int64(1)      
		limiter := swredis.NewLimiter(limiterKey, window, limit, client, swredis.WithClock(mockNowFuncSWC(t)))

		// Req 1: Allow
		allowed, _ := limiter.Allow(ctx, "user3")
		if !allowed { t.Fatal("Req 1: Denied") }

		// Req 2: Deny
		advanceMockTimeSWC(500 * time.Millisecond)
		allowed, _ = limiter.Allow(ctx, "user3")
		if allowed { t.Fatal("Req 2: Allowed, should deny") }

		// Advance time well past the window
		advanceMockTimeSWC(window + 1*time.Second) // Total elapsed > window

		// Req 3: Allow (new window)
		allowed, _ = limiter.Allow(ctx, "user3")
		if !allowed { t.Fatal("Req 3: Denied after window reset") }
		
		redisKey := fmt.Sprintf("%s:%s", limiterKey, "user3")
		count, _ := client.ZCard(ctx, redisKey).Result()
		if count != 1 { // After reset and one new request
			t.Errorf("Expected count in Redis to be 1 after reset, got %d", count)
		}
		resetMockTimeSWC()
	})

	t.Run("DifferentIdentifiersSWC", func(t *testing.T) {
		limiterKey := baseLimiterKey + "_diffids_swc"
		redistest.CleanupRedisKeys(t, client, limiterKey, "")
		defer redistest.CleanupRedisKeys(t, client, limiterKey, "")

		resetMockTimeSWC()
		window := 5 * time.Second
		limit := int64(1)
		limiter := swredis.NewLimiter(limiterKey, window, limit, client, swredis.WithClock(mockNowFuncSWC(t)))

		// User A
		allowedA1, _ := limiter.Allow(ctx, "userA_swc")
		if !allowedA1 { t.Fatal("UserA SWC Req1: Should allow") }
		allowedA2, _ := limiter.Allow(ctx, "userA_swc")
		if allowedA2 { t.Fatal("UserA SWC Req2: Should deny") }

		// User B (independent)
		advanceMockTimeSWC(100*time.Millisecond) // Ensure different timestamps for different users if script relies on exact now for member score
		allowedB1, _ := limiter.Allow(ctx, "userB_swc")
		if !allowedB1 { t.Fatal("UserB SWC Req1: Should allow") }
		allowedB2, _ := limiter.Allow(ctx, "userB_swc")
		if allowedB2 { t.Fatal("UserB SWC Req2: Should deny") }
		
		resetMockTimeSWC()
	})
}
