package lbredis_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	// "github.com/go-redis/redis/v8" // Likely unused if errors handled via strings or errors.Is
	lbredis "learn.ratelimiter/internal/leakybucket/redis"
	"learn.ratelimiter/internal/testharness/redistest"
)

// Leaky bucket state for unmarshalling from Redis (if needed for direct inspection)
type leakyBucketState struct {
	CurrentLevel float64   `json:"currentLevel"`
	LastLeak     int64     `json:"lastLeak"` // Assuming stored as UnixMilli
}

// Shared mock time for controlling time in tests
var mockTimeLB = time.Date(2024, time.January, 1, 12, 0, 0, 0, time.UTC)

func mockNowFuncLB(t *testing.T) func() time.Time {
	currentTime := mockTimeLB
	return func() time.Time {
		// t.Logf("LeakyBucket mockNowFunc called, returning: %v", currentTime)
		return currentTime
	}
}

func advanceMockTimeLB(duration time.Duration) {
	mockTimeLB = mockTimeLB.Add(duration)
}

func resetMockTimeLB() {
	mockTimeLB = time.Date(2024, time.January, 1, 12, 0, 0, 0, time.UTC)
}


func TestLeakyBucketRedis_Integration(t *testing.T) {
	client := redistest.SetupRedisClient(t)
	defer client.Close()

	baseLimiterKey := "test_lb_integration"
	ctx := context.Background()

	// This pattern prefix should match how keys are actually formed by the limiter.
	// The leaky bucket limiter uses: fmt.Sprintf("leaky_bucket:%s:%s", l.key, identifier)
	// So, if l.key is "limiterKey", the actual redis key is "leaky_bucket:limiterKey:identifier".
	// Our CleanupRedisKeys helper scans for "patternPrefix:limiterKeySuffix:*"
	// So, patternPrefix will be "leaky_bucket", and limiterKeySuffix will be the specific test's limiterKey.
	const leakyBucketRedisKeyPrefix = "leaky_bucket"


	t.Run("BasicAllowanceAndDenial", func(t *testing.T) {
		limiterKey := baseLimiterKey + "_basic"
		redistest.CleanupRedisKeys(t, client, leakyBucketRedisKeyPrefix, limiterKey)
		defer redistest.CleanupRedisKeys(t, client, leakyBucketRedisKeyPrefix, limiterKey)
		
		resetMockTimeLB()
		rate := 2 // tokens per second
		capacity := 3
		// Use injected clock for predictable LastLeak times in stored state
		limiter := lbredis.NewLimiter(limiterKey, rate, capacity, client, lbredis.WithClock(mockNowFuncLB(t)))

		// Req 1, 2, 3: Allow
		for i := 1; i <= capacity; i++ {
			allowed, err := limiter.Allow(ctx, "user1")
			if err != nil {
				t.Fatalf("Allow %d: unexpected error: %v", i, err)
			}
			if !allowed {
				t.Fatalf("Allow %d: should be allowed", i)
			}
		}

		// Req 4: Deny (capacity is 3)
		allowed, err := limiter.Allow(ctx, "user1")
		if err != nil {
			t.Fatalf("Allow 4: unexpected error: %v", err)
		}
		if allowed {
			t.Fatal("Allow 4: should be denied, capacity reached")
		}
		
		// Verify state in Redis (optional, but good for integration)
		redisKey := fmt.Sprintf("%s:%s:%s", leakyBucketRedisKeyPrefix, limiterKey, "user1")
		valStr, err := client.Get(ctx, redisKey).Result()
		if err != nil {
			t.Fatalf("Error getting key %s from Redis: %v", redisKey, err)
		}
		var state leakyBucketState
		if err := json.Unmarshal([]byte(valStr), &state); err != nil {
			t.Fatalf("Error unmarshalling state: %v. Value: %s", err, valStr)
		}
		if state.CurrentLevel != float64(capacity) {
			t.Errorf("Expected current level to be %d, got %f", capacity, state.CurrentLevel)
		}
		if state.LastLeak != mockTimeLB.UnixNano()/int64(time.Millisecond) {
			t.Errorf("Expected lastLeak to be %d, got %d", mockTimeLB.UnixNano()/int64(time.Millisecond), state.LastLeak)
		}
	})

	t.Run("TokenLeakOverTime", func(t *testing.T) {
		limiterKey := baseLimiterKey + "_leak"
		redistest.CleanupRedisKeys(t, client, leakyBucketRedisKeyPrefix, limiterKey)
		defer redistest.CleanupRedisKeys(t, client, leakyBucketRedisKeyPrefix, limiterKey)

		resetMockTimeLB()
		rate := 1 // 1 token per second
		capacity := 2
		limiter := lbredis.NewLimiter(limiterKey, rate, capacity, client, lbredis.WithClock(mockNowFuncLB(t)))

		// Fill the bucket (2 requests)
		limiter.Allow(ctx, "user2") // Allowed at mockTimeLB
		limiter.Allow(ctx, "user2") // Allowed at mockTimeLB, bucket full, level 2

		// Deny (still at mockTimeLB, bucket full)
		allowed, _ := limiter.Allow(ctx, "user2")
		if allowed { t.Fatal("Should be denied, bucket is full initially") }
		
		// Advance time by 1 second (1 token should leak)
		advanceMockTimeLB(1 * time.Second)
		t.Logf("Advanced mockTimeLB to: %v", mockTimeLB)

		// Allow (1 token leaked, 1 spot available, level becomes 2 again)
		allowed, err := limiter.Allow(ctx, "user2")
		if err != nil {
			t.Fatalf("Allow after 1s: unexpected error: %v", err)
		}
		if !allowed {
			t.Fatal("Allow after 1s: should be allowed after 1 token leaked")
		}

		// Deny (bucket full again)
		allowed, _ = limiter.Allow(ctx, "user2")
		if allowed { t.Fatal("Should be denied again, bucket is full after refill and consumption") }

		// Advance time by 2 seconds (2 tokens should leak, bucket empty, then 1 spot available)
		advanceMockTimeLB(2 * time.Second)
		t.Logf("Advanced mockTimeLB further to: %v", mockTimeLB)
		
		// Allow (bucket empty, 1 token added, level becomes 1)
		allowed, err = limiter.Allow(ctx, "user2")
		if err != nil {
			t.Fatalf("Allow after 2s: unexpected error: %v", err)
		}
		if !allowed {
			t.Fatal("Allow after 2s: should be allowed as bucket should be empty then one taken")
		}
		
		// Verify state
		redisKey := fmt.Sprintf("%s:%s:%s", leakyBucketRedisKeyPrefix, limiterKey, "user2")
		valStr, errGet := client.Get(ctx, redisKey).Result()
		if errGet != nil {
			t.Fatalf("Error getting key %s from Redis: %v", redisKey, errGet)
		}
		var state leakyBucketState
		json.Unmarshal([]byte(valStr), &state)
		// Expected level: after 2s leak, bucket was empty (level 0). Then 1 request came. So level is 1.
		if state.CurrentLevel != 1.0 {
			t.Errorf("Expected current level 1.0, got %f. State: %+v", state.CurrentLevel, state)
		}
		resetMockTimeLB()
	})
	
	t.Run("DifferentIdentifiers", func(t *testing.T) {
		limiterKey := baseLimiterKey + "_diffids_lb"
		redistest.CleanupRedisKeys(t, client, leakyBucketRedisKeyPrefix, limiterKey)
		defer redistest.CleanupRedisKeys(t, client, leakyBucketRedisKeyPrefix, limiterKey)

		resetMockTimeLB()
		rate := 1 
		capacity := 1
		limiter := lbredis.NewLimiter(limiterKey, rate, capacity, client, lbredis.WithClock(mockNowFuncLB(t)))

		// User A
		allowedA1, _ := limiter.Allow(ctx, "userA_lb")
		if !allowedA1 { t.Fatal("UserA Lb Req1: Should allow") }
		allowedA2, _ := limiter.Allow(ctx, "userA_lb")
		if allowedA2 { t.Fatal("UserA Lb Req2: Should deny") }

		// User B (independent)
		allowedB1, _ := limiter.Allow(ctx, "userB_lb")
		if !allowedB1 { t.Fatal("UserB Lb Req1: Should allow") }
		allowedB2, _ := limiter.Allow(ctx, "userB_lb")
		if allowedB2 { t.Fatal("UserB Lb Req2: Should deny") }
		
		resetMockTimeLB()
	})
}
