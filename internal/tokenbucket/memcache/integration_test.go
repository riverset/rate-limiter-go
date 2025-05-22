package tbmemcache_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	tbmemcache "learn.ratelimiter/internal/tokenbucket/memcache"
	"learn.ratelimiter/internal/testharness/memcachetest"
)

// tokenBucketStateTest is a local copy for unmarshalling if the main one is unexported
type tokenBucketStateTest struct {
	Tokens     int64     `json:"tokens"`
	LastRefill time.Time `json:"last_refill"`
}


func TestTokenBucketMemcache_Integration(t *testing.T) {
	mcClient := memcachetest.SetupMemcachedClient(t)
	adapter := memcachetest.NewMemcacheClientAdapter(mcClient)

	baseLimiterKey := "test_tb_integration"
	ctx := context.Background()

	t.Run("NewBucket_AllowsAndDecrements", func(t *testing.T) {
		limiterKey := baseLimiterKey + "_newbucket"
		id1Key := fmt.Sprintf("token_bucket:%s:%s", limiterKey, "user1") // Key as stored by limiter
		memcachetest.CleanupMemcachedKeys(t, mcClient, []string{id1Key})
		defer memcachetest.CleanupMemcachedKeys(t, mcClient, []string{id1Key})

		rate := 10
		capacity := 5
		
		// Use controlled time
		startTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
		mockNow := func() time.Time { return startTime }

		limiter := tbmemcache.NewLimiter(limiterKey, rate, capacity, adapter, tbmemcache.WithClock(mockNow))

		allowed, err := limiter.Allow(ctx, "user1")
		if err != nil { t.Fatalf("Allow: unexpected error: %v", err) }
		if !allowed { t.Fatal("Allow: should be allowed for new bucket") }

		// Verify state in Memcached
		item, err := mcClient.Get(id1Key)
		if err != nil { t.Fatalf("Failed to get key %s: %v", id1Key, err) }
		var state tokenBucketStateTest
		if err := json.Unmarshal(item.Value, &state); err != nil {
			t.Fatalf("Failed to unmarshal state: %v", err)
		}
		if state.Tokens != int64(capacity-1) {
			t.Errorf("Expected tokens to be %d, got %d", capacity-1, state.Tokens)
		}
		if !state.LastRefill.Equal(startTime) {
			t.Errorf("Expected LastRefill to be %v, got %v", startTime, state.LastRefill)
		}
	})

	t.Run("TokenRefillOverTime", func(t *testing.T) {
		limiterKey := baseLimiterKey + "_refill"
		id1Key := fmt.Sprintf("token_bucket:%s:%s", limiterKey, "user2")
		memcachetest.CleanupMemcachedKeys(t, mcClient, []string{id1Key})
		defer memcachetest.CleanupMemcachedKeys(t, mcClient, []string{id1Key})

		rate := 1 // 1 token per second
		capacity := 2
		
		currentTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
		mockNow := func() time.Time { return currentTime }
		
		limiter := tbmemcache.NewLimiter(limiterKey, rate, capacity, adapter, tbmemcache.WithClock(mockNow))

		// Consume initial tokens (capacity is 2)
		limiter.Allow(ctx, "user2") // Allowed, tokens = 1, LR = 12:00:00
		limiter.Allow(ctx, "user2") // Allowed, tokens = 0, LR = 12:00:00
		
		// Third request should be denied
		allowed, _ := limiter.Allow(ctx, "user2") // Denied, tokens = 0, LR = 12:00:00
		if allowed { t.Fatal("Should be denied, bucket empty before refill") }

		// Advance time by 1 second (1 token should refill)
		currentTime = currentTime.Add(1 * time.Second) // Time is 12:00:01

		// Next request should be allowed
		allowed, err := limiter.Allow(ctx, "user2")
		if err != nil { t.Fatalf("Allow after 1s: unexpected error: %v", err) }
		if !allowed { t.Fatal("Allow after 1s: should be allowed after 1 token refilled") }

		// Verify state: Tokens should be 0 (1 refilled - 1 consumed), LastRefill = 12:00:01
		item, _ := mcClient.Get(id1Key)
		var state tokenBucketStateTest
		json.Unmarshal(item.Value, &state)
		if state.Tokens != 0 {
			t.Errorf("Expected tokens to be 0 after refill and consumption, got %d. State: %+v", state.Tokens, state)
		}
		if !state.LastRefill.Equal(currentTime) {
			t.Errorf("Expected LastRefill to be %v, got %v", currentTime, state.LastRefill)
		}

		// Advance time by another 2 seconds (should refill to capacity, 2 tokens)
		currentTime = currentTime.Add(2 * time.Second) // Time is 12:00:03

		// Request should be allowed
		allowed, _ = limiter.Allow(ctx, "user2")
		if !allowed { t.Fatal("Allow after 2s: should be allowed") }
		// Verify state: Tokens should be 1 (capacity 2 - 1 consumed)
		item, _ = mcClient.Get(id1Key)
		json.Unmarshal(item.Value, &state)
		if state.Tokens != int64(capacity-1) { // Max capacity is 2, 2 refilled, 1 consumed -> 1 left
			t.Errorf("Expected tokens to be %d after full refill and consumption, got %d. State: %+v", capacity-1, state.Tokens, state)
		}
	})
	
	t.Run("DenialWhenNoTokensAndNoRefill", func(t *testing.T) {
		limiterKey := baseLimiterKey + "_notokens"
		id1Key := fmt.Sprintf("token_bucket:%s:%s", limiterKey, "user3")
		memcachetest.CleanupMemcachedKeys(t, mcClient, []string{id1Key})
		defer memcachetest.CleanupMemcachedKeys(t, mcClient, []string{id1Key})

		rate := 10 
		capacity := 1
		
		currentTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
		mockNow := func() time.Time { return currentTime }
		limiter := tbmemcache.NewLimiter(limiterKey, rate, capacity, adapter, tbmemcache.WithClock(mockNow))

		// Consume the only token
		limiter.Allow(ctx, "user3") // Allowed, tokens = 0

		// Try again immediately (no time passes for refill)
		allowed, err := limiter.Allow(ctx, "user3")
		if err != nil { t.Fatalf("Allow: unexpected error: %v", err) }
		if allowed { t.Fatal("Allow: should be denied, no tokens and no refill time") }
	})

	t.Run("DifferentIdentifiers", func(t *testing.T) {
		limiterKey := baseLimiterKey + "_diffids_tb"
		userAKey := fmt.Sprintf("token_bucket:%s:%s", limiterKey, "userA_tb")
		userBKey := fmt.Sprintf("token_bucket:%s:%s", limiterKey, "userB_tb")
		memcachetest.CleanupMemcachedKeys(t, mcClient, []string{userAKey, userBKey})
		defer memcachetest.CleanupMemcachedKeys(t, mcClient, []string{userAKey, userBKey})

		rate := 1
		capacity := 1
		limiter := tbmemcache.NewLimiter(limiterKey, rate, capacity, adapter) // Using real time for simplicity here

		// User A
		allowedA1, _ := limiter.Allow(ctx, "userA_tb")
		if !allowedA1 { t.Fatal("UserA TB Req1: Should allow") }
		allowedA2, _ := limiter.Allow(ctx, "userA_tb")
		if allowedA2 { t.Fatal("UserA TB Req2: Should deny") }

		// User B (independent)
		allowedB1, _ := limiter.Allow(ctx, "userB_tb")
		if !allowedB1 { t.Fatal("UserB TB Req1: Should allow") }
		allowedB2, _ := limiter.Allow(ctx, "userB_tb")
		if allowedB2 { t.Fatal("UserB TB Req2: Should deny") }
	})
}
