package fcmemcache_test

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	fcmemcache "learn.ratelimiter/internal/fixedcounter/memcache"
	"learn.ratelimiter/internal/testharness/memcachetest"
)

func TestFixedCounterMemcache_Integration(t *testing.T) {
	mcClient := memcachetest.SetupMemcachedClient(t)
	adapter := memcachetest.NewMemcacheClientAdapter(mcClient)
	
	baseLimiterKey := "test_fc_integration"
	ctx := context.Background()

	t.Run("BasicAllowanceAndDenial", func(t *testing.T) {
		limiterKey := baseLimiterKey + "_basic"
		// Cleanup function needs to know all keys used.
		// For fixed counter, it's one key per identifier.
		id1Key := fmt.Sprintf("%s:%s", limiterKey, "user1")
		memcachetest.CleanupMemcachedKeys(t, mcClient, []string{id1Key})
		defer memcachetest.CleanupMemcachedKeys(t, mcClient, []string{id1Key})

		window := 3 * time.Second 
		limit := 2
		
		limiter := fcmemcache.NewLimiter(adapter, limiterKey, window, limit) 

		// Request 1: Allow
		allowed, err := limiter.Allow(ctx, "user1")
		if err != nil { t.Fatalf("Allow 1: unexpected error: %v", err) }
		if !allowed { t.Fatal("Allow 1: should be allowed") }

		// Request 2: Allow
		allowed, err = limiter.Allow(ctx, "user1")
		if err != nil { t.Fatalf("Allow 2: unexpected error: %v", err) }
		if !allowed { t.Fatal("Allow 2: should be allowed") }

		// Request 3: Deny (limit is 2)
		allowed, err = limiter.Allow(ctx, "user1")
		if err != nil { t.Fatalf("Allow 3: unexpected error: %v", err) }
		if allowed { t.Fatal("Allow 3: should be denied, limit reached") }
	})

	t.Run("WindowReset", func(t *testing.T) {
		limiterKey := baseLimiterKey + "_reset"
		id1Key := fmt.Sprintf("%s:%s", limiterKey, "user2")
		memcachetest.CleanupMemcachedKeys(t, mcClient, []string{id1Key})
		defer memcachetest.CleanupMemcachedKeys(t, mcClient, []string{id1Key})
		
		window := 2 * time.Second 
		limit := 1      
		limiter := fcmemcache.NewLimiter(adapter, limiterKey, window, limit)

		// Request 1: Allow
		allowed, err := limiter.Allow(ctx, "user2")
		if err != nil { t.Fatalf("Allow 1: unexpected error: %v", err) }
		if !allowed { t.Fatal("Allow 1: should be allowed") }
		
		// Request 2: Deny (still within window, limit 1)
		allowed, err = limiter.Allow(ctx, "user2")
		if err != nil { t.Fatalf("Allow 2: unexpected error: %v", err) }
		if allowed { t.Fatal("Allow 2: should be denied") }

		// Wait for the window to pass
		t.Logf("Waiting for %v for window to reset...", window + 500*time.Millisecond)
		time.Sleep(window + 500*time.Millisecond)

		// Request 3: Allow (new window)
		allowed, err = limiter.Allow(ctx, "user2")
		if err != nil { t.Fatalf("Allow 3 (after sleep): unexpected error: %v", err) }
		if !allowed { t.Fatal("Allow 3 (after sleep): should be allowed after window reset") }
		
		// Verify count in Memcached
		item, err := mcClient.Get(id1Key)
		if err != nil {
			t.Fatalf("Failed to get key %s after reset: %v", id1Key, err)
		}
		count, _ := strconv.Atoi(string(item.Value))
		if count != 1 {
			t.Errorf("Expected count in Memcached to be 1 after reset, got %d", count)
		}
	})

	t.Run("DifferentIdentifiers", func(t *testing.T) {
		limiterKey := baseLimiterKey + "_diffids"
		userAKey := fmt.Sprintf("%s:%s", limiterKey, "userA")
		userBKey := fmt.Sprintf("%s:%s", limiterKey, "userB")
		memcachetest.CleanupMemcachedKeys(t, mcClient, []string{userAKey, userBKey})
		defer memcachetest.CleanupMemcachedKeys(t, mcClient, []string{userAKey, userBKey})

		window := 5 * time.Second
		limit := 1
		limiter := fcmemcache.NewLimiter(adapter, limiterKey, window, limit)

		// User A: Allow
		allowedA, errA := limiter.Allow(ctx, "userA")
		if errA != nil || !allowedA { t.Fatal("userA should be allowed") }
		// User A: Deny
		allowedA2, errA2 := limiter.Allow(ctx, "userA")
		if errA2 != nil || allowedA2 { t.Fatal("userA should be denied on second attempt") }

		// User B: Allow (independent)
		allowedB, errB := limiter.Allow(ctx, "userB")
		if errB != nil || !allowedB { t.Fatal("userB should be allowed") }
	})
}
