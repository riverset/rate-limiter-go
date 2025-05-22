package fcmemcache_test

import (
	"context"
	"errors"
	"fmt"
	"strings" // Add missing import
	"testing"
	"time"

	"github.com/bradfitz/gomemcache/memcache"

	"learn.ratelimiter/internal/memcacheiface"
	fcmemcache "learn.ratelimiter/internal/fixedcounter/memcache"
	"learn.ratelimiter/types"
)

// mockMemcacheClient_FC is a mock implementation for FixedCounter tests.
type mockMemcacheClient_FC struct {
	memcacheiface.Client // Embed the interface for any pass-through if needed

	AddFunc       func(item *memcache.Item) error
	IncrementFunc func(key string, delta uint64) (newValue uint64, err error)
	GetFunc       func(key string) (*memcache.Item, error) // For other limiters, if interface is shared
	SetFunc       func(item *memcache.Item) error       // For other limiters

	// Store calls for assertions if needed
	addCalls       map[string]*memcache.Item
	incrementCalls map[string]uint64
}

func NewMockMemcacheClient_FC() *mockMemcacheClient_FC {
	return &mockMemcacheClient_FC{
		addCalls:       make(map[string]*memcache.Item),
		incrementCalls: make(map[string]uint64),
	}
}

func (m *mockMemcacheClient_FC) Add(item *memcache.Item) error {
	if m.AddFunc != nil {
		return m.AddFunc(item)
	}
	m.addCalls[item.Key] = item
	return nil // Default: success
}

func (m *mockMemcacheClient_FC) Increment(key string, delta uint64) (uint64, error) {
	if m.IncrementFunc != nil {
		return m.IncrementFunc(key, delta)
	}
	m.incrementCalls[key] += delta
	return m.incrementCalls[key], nil // Default: success
}

func (m *mockMemcacheClient_FC) Get(key string) (*memcache.Item, error) {
	if m.GetFunc != nil {
		return m.GetFunc(key)
	}
	return nil, memcache.ErrCacheMiss
}

func (m *mockMemcacheClient_FC) Set(item *memcache.Item) error {
	if m.SetFunc != nil {
		return m.SetFunc(item)
	}
	return nil
}

func TestNewLimiter_FixedCounterMemcache(t *testing.T) {
	mockClient := NewMockMemcacheClient_FC()
	keyPrefix := "test_fc_new"
	window := 60 * time.Second
	limit := 10

	limiter := fcmemcache.NewLimiter(mockClient, keyPrefix, window, limit)
	if limiter == nil {
		t.Fatal("NewLimiter returned nil")
	}
	_, ok := limiter.(types.Limiter)
	if !ok {
		t.Fatalf("NewLimiter did not return a types.Limiter")
	}
}

func TestAllow_FixedCounterMemcache(t *testing.T) {
	ctx := context.Background()
	keyPrefix := "test_fc_allow"
	window := 60 * time.Second
	limit := 3
	identifier := "user1"
	expectedMemcacheKey := fmt.Sprintf("%s:%s", keyPrefix, identifier)
	expectedExpiry := int32(window.Seconds())

	t.Run("SuccessfulAllowance_FirstTime", func(t *testing.T) {
		mockClient := NewMockMemcacheClient_FC()
		limiter := fcmemcache.NewLimiter(mockClient, keyPrefix, window, limit)

		mockClient.AddFunc = func(item *memcache.Item) error {
			if item.Key != expectedMemcacheKey {
				t.Errorf("Add: Expected key %s, got %s", expectedMemcacheKey, item.Key)
			}
			if string(item.Value) != "1" {
				t.Errorf("Add: Expected value '1', got '%s'", string(item.Value))
			}
			if item.Expiration != expectedExpiry {
				t.Errorf("Add: Expected expiration %d, got %d", expectedExpiry, item.Expiration)
			}
			return nil // Success
		}

		allowed, err := limiter.Allow(ctx, identifier)
		if err != nil {
			t.Fatalf("Allow failed: %v", err)
		}
		if !allowed {
			t.Fatal("Request unexpectedly denied on first attempt")
		}
	})

	t.Run("SuccessfulAllowance_IncrementExisting", func(t *testing.T) {
		mockClient := NewMockMemcacheClient_FC()
		limiter := fcmemcache.NewLimiter(mockClient, keyPrefix, window, limit)
		currentCount := uint64(1) // This will be the value *before* increment for this call

		mockClient.AddFunc = func(item *memcache.Item) error {
			return memcache.ErrNotStored // Simulate key already exists
		}
		mockClient.IncrementFunc = func(key string, delta uint64) (uint64, error) {
			if key != expectedMemcacheKey {
				t.Errorf("Increment: Expected key %s, got %s", expectedMemcacheKey, key)
			}
			if delta != 1 {
				t.Errorf("Increment: Expected delta 1, got %d", delta)
			}
			currentCount += delta
			return currentCount, nil
		}

		// First call (denied by Add, success by Increment)
		allowed, err := limiter.Allow(ctx, identifier) // Count becomes 2
		if err != nil {
			t.Fatalf("Allow (1) failed: %v", err)
		}
		if !allowed {
			t.Fatal("Allow (1): Request unexpectedly denied")
		}
		if currentCount != 2 {
			t.Fatalf("Expected count to be 2, got %d", currentCount)
		}
		
		// Second call (denied by Add, success by Increment)
		allowed, err = limiter.Allow(ctx, identifier) // Count becomes 3
		if err != nil {
			t.Fatalf("Allow (2) failed: %v", err)
		}
		if !allowed {
			t.Fatal("Allow (2): Request unexpectedly denied")
		}
		if currentCount != 3 {
			t.Fatalf("Expected count to be 3, got %d", currentCount)
		}
	})
	
	t.Run("Denial_OverLimit", func(t *testing.T) {
		mockClient := NewMockMemcacheClient_FC()
		limiter := fcmemcache.NewLimiter(mockClient, keyPrefix, window, limit) // limit is 3
		currentCount := uint64(limit -1) // Start at 2, next increment will be 3 (allowed)

		mockClient.AddFunc = func(item *memcache.Item) error {
			return memcache.ErrNotStored // Simulate key already exists
		}
		mockClient.IncrementFunc = func(key string, delta uint64) (uint64, error) {
			currentCount += delta
			return currentCount, nil
		}

		// Call 1: count becomes limit (3), should be allowed
		allowed, err := limiter.Allow(ctx, identifier)
		if err != nil { t.Fatalf("Allow (1) failed: %v", err) }
		if !allowed { t.Fatal("Allow (1): Expected to be allowed") }
		if currentCount != uint64(limit) {t.Fatalf("Count should be %d, got %d", limit, currentCount)}


		// Call 2: count becomes limit + 1 (4), should be denied
		allowed, err = limiter.Allow(ctx, identifier)
		if err != nil { t.Fatalf("Allow (2) failed: %v", err) }
		if allowed { t.Fatal("Allow (2): Expected to be denied (over limit)") }
		if currentCount != uint64(limit+1) {t.Fatalf("Count should be %d, got %d", limit+1, currentCount)}

	})

	t.Run("MemcacheAddError", func(t *testing.T) {
		mockClient := NewMockMemcacheClient_FC()
		limiter := fcmemcache.NewLimiter(mockClient, keyPrefix, window, limit)
		expectedErr := errors.New("memcache Add error")

		mockClient.AddFunc = func(item *memcache.Item) error {
			return expectedErr
		}

		_, err := limiter.Allow(ctx, identifier)
		if err == nil {
			t.Fatal("Expected an error from Add but got nil")
		}
		if !errors.Is(err, expectedErr) { // errors.Is for wrapped errors
			// Check if the original error is contained if it's wrapped by fmt.Errorf
			if !strings.Contains(err.Error(), expectedErr.Error()){
				t.Fatalf("Expected error containing '%s', got '%v'", expectedErr.Error(), err)
			}
		}
	})

	t.Run("MemcacheIncrementError", func(t *testing.T) {
		mockClient := NewMockMemcacheClient_FC()
		limiter := fcmemcache.NewLimiter(mockClient, keyPrefix, window, limit)
		expectedErr := errors.New("memcache Increment error")

		mockClient.AddFunc = func(item *memcache.Item) error {
			return memcache.ErrNotStored // Simulate key exists to trigger Increment
		}
		mockClient.IncrementFunc = func(key string, delta uint64) (uint64, error) {
			return 0, expectedErr
		}

		_, err := limiter.Allow(ctx, identifier)
		if err == nil {
			t.Fatal("Expected an error from Increment but got nil")
		}
		if !errors.Is(err, expectedErr) {
			if !strings.Contains(err.Error(), expectedErr.Error()){
				t.Fatalf("Expected error containing '%s', got '%v'", expectedErr.Error(), err)
			}
		}
	})
}
