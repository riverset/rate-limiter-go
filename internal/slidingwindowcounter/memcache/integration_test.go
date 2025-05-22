package swcmemcache_test

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"testing"
	"time"

	swcmemcache "learn.ratelimiter/internal/slidingwindowcounter/memcache"
	"learn.ratelimiter/internal/testharness/memcachetest"
)

// requestTimestampsTest is a local copy for unmarshalling if the main one is unexported
type requestTimestampsTest struct {
	Timestamps []int64 `json:"timestamps"`
}

func TestSlidingWindowMemcache_Integration(t *testing.T) {
	mcClient := memcachetest.SetupMemcachedClient(t)
	adapter := memcachetest.NewMemcacheClientAdapter(mcClient)

	baseLimiterKey := "test_swc_integration"
	ctx := context.Background()

	t.Run("BasicAllowanceAndDenial", func(t *testing.T) {
		limiterKey := baseLimiterKey + "_basic"
		id1Key := fmt.Sprintf("%s:%s", limiterKey, "user1")
		memcachetest.CleanupMemcachedKeys(t, mcClient, []string{id1Key})
		defer memcachetest.CleanupMemcachedKeys(t, mcClient, []string{id1Key})

		window := 3 * time.Second
		limit := 2
		
		// Use real time for basic test
		limiter := swcmemcache.NewLimiter(adapter, limiterKey, window, limit)

		// Request 1: Allow
		allowed, err := limiter.Allow(ctx, "user1")
		if err != nil { t.Fatalf("Allow 1: unexpected error: %v", err) }
		if !allowed { t.Fatal("Allow 1: should be allowed") }

		// Request 2: Allow
		time.Sleep(100 * time.Millisecond) // Ensure distinct timestamp if needed by underlying logic
		allowed, err = limiter.Allow(ctx, "user1")
		if err != nil { t.Fatalf("Allow 2: unexpected error: %v", err) }
		if !allowed { t.Fatal("Allow 2: should be allowed") }

		// Request 3: Deny
		time.Sleep(100 * time.Millisecond)
		allowed, err = limiter.Allow(ctx, "user1")
		if err != nil { t.Fatalf("Allow 3: unexpected error: %v", err) }
		if allowed { t.Fatal("Allow 3: should be denied") }
	})

	t.Run("WindowSlidingAndPruning", func(t *testing.T) {
		limiterKey := baseLimiterKey + "_slideprune"
		id1Key := fmt.Sprintf("%s:%s", limiterKey, "user2")
		memcachetest.CleanupMemcachedKeys(t, mcClient, []string{id1Key})
		defer memcachetest.CleanupMemcachedKeys(t, mcClient, []string{id1Key})

		window := 2 * time.Second
		limit := 2
		
		// Use a controllable clock for this test
		currentTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
		mockNow := func() time.Time { return currentTime }
		
		limiter := swcmemcache.NewLimiter(adapter, limiterKey, window, limit, swcmemcache.WithClock(mockNow))

		// Request 1 (Time: 12:00:00) - Allowed
		allowed, err := limiter.Allow(ctx, "user2")
		if err != nil { t.Fatalf("T0 Allow 1: error: %v", err) }
		if !allowed { t.Fatal("T0 Allow 1: should allow") }

		// Request 2 (Time: 12:00:00, after 500ms from first) - Allowed
		currentTime = currentTime.Add(500 * time.Millisecond) // 12:00:00.500
		allowed, err = limiter.Allow(ctx, "user2")
		if err != nil { t.Fatalf("T0.5 Allow 2: error: %v", err) }
		if !allowed { t.Fatal("T0.5 Allow 2: should allow") }

		// Request 3 (Time: 12:00:00.500, after another 500ms) - Denied
		currentTime = currentTime.Add(500 * time.Millisecond) // 12:00:01.000
		allowed, err = limiter.Allow(ctx, "user2")
		if err != nil { t.Fatalf("T1 Allow 3: error: %v", err) }
		if allowed { t.Fatal("T1 Allow 3: should deny, limit reached") }

		// Advance time so the first request (at 12:00:00) expires
		// Window is 2s. Current time is 12:00:01.000.
		// First request at 12:00:00.000 should expire when current time > 12:00:02.000
		currentTime = currentTime.Add(1*time.Second + 100*time.Millisecond) // Time is now 12:00:02.100

		// Request 4 (Time: 12:00:02.100) - Should be allowed as first request expired
		allowed, err = limiter.Allow(ctx, "user2")
		if err != nil { t.Fatalf("T2.1 Allow 4: error: %v", err) }
		if !allowed {
			// Check state in memcache
			item, _ := mcClient.Get(id1Key)
			var state requestTimestampsTest
			json.Unmarshal(item.Value, &state)
			t.Fatalf("T2.1 Allow 4: should allow as first request expired. Current state: %+v", state)
		}

		// Verify content in memcache
		item, err := mcClient.Get(id1Key)
		if err != nil { t.Fatalf("Failed to get key %s: %v", id1Key, err) }
		var finalState requestTimestampsTest
		json.Unmarshal(item.Value, &finalState)

		if len(finalState.Timestamps) != 2 {
			t.Errorf("Expected 2 timestamps after pruning and adding, got %d. Timestamps: %v", len(finalState.Timestamps), finalState.Timestamps)
		}
		// Expected timestamps: 12:00:00.500 (as UnixMilli) and 12:00:02.100 (as UnixMilli)
		// Sort them to ensure order for comparison if needed, though the code sorts already.
		sort.Slice(finalState.Timestamps, func(i, j int) bool { return finalState.Timestamps[i] < finalState.Timestamps[j] })
		
		expectedTs1 := time.Date(2024, 1, 1, 12, 0, 0, 500*1e6, time.UTC).UnixMilli()
		expectedTs2 := currentTime.UnixMilli() // 12:00:02.100

		if finalState.Timestamps[0] != expectedTs1 || finalState.Timestamps[1] != expectedTs2 {
			t.Errorf("Timestamps not as expected. Got: %v, Expected approx: [%d, %d]", finalState.Timestamps, expectedTs1, expectedTs2)
		}
	})
	
	t.Run("DifferentIdentifiers", func(t *testing.T) {
		limiterKey := baseLimiterKey + "_diffids_swc"
		userAKey := fmt.Sprintf("%s:%s", limiterKey, "userA_swc")
		userBKey := fmt.Sprintf("%s:%s", limiterKey, "userB_swc")
		memcachetest.CleanupMemcachedKeys(t, mcClient, []string{userAKey, userBKey})
		defer memcachetest.CleanupMemcachedKeys(t, mcClient, []string{userAKey, userBKey})

		window := 5 * time.Second
		limit := 1
		limiter := swcmemcache.NewLimiter(adapter, limiterKey, window, limit)

		// User A
		allowedA1, _ := limiter.Allow(ctx, "userA_swc")
		if !allowedA1 { t.Fatal("UserA SWC Req1: Should allow") }
		allowedA2, _ := limiter.Allow(ctx, "userA_swc")
		if allowedA2 { t.Fatal("UserA SWC Req2: Should deny") }

		// User B (independent)
		time.Sleep(100 * time.Millisecond) // Ensure a different timestamp for User B's first request
		allowedB1, _ := limiter.Allow(ctx, "userB_swc")
		if !allowedB1 { t.Fatal("UserB SWC Req1: Should allow") }
		allowedB2, _ := limiter.Allow(ctx, "userB_swc")
		if allowedB2 { t.Fatal("UserB SWC Req2: Should deny") }
	})
}
