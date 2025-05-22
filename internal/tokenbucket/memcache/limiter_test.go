package tbmemcache_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv" // Add missing import
	"strings" // Add missing import
	"testing"
	"time"

	"github.com/bradfitz/gomemcache/memcache"

	"learn.ratelimiter/internal/memcacheiface"
	tbmemcache "learn.ratelimiter/internal/tokenbucket/memcache"
	"learn.ratelimiter/types"
)

// mockTime is a fixed point in time for testing.
var mockTime = time.Date(2024, time.January, 1, 12, 0, 0, 0, time.UTC)

// mockNowFunc returns a function that returns the mockTime.
func mockNowFunc() func() time.Time {
	return func() time.Time {
		return mockTime
	}
}

// mockMemcacheClient is a mock implementation of memcacheiface.Client for testing.
type mockMemcacheClient struct {
	memcacheiface.Client // Embed interface

	GetFunc       func(key string) (*memcache.Item, error)
	SetFunc       func(item *memcache.Item) error
	AddFunc       func(item *memcache.Item) error
	IncrementFunc func(key string, delta uint64) (newValue uint64, err error)

	setCalls map[string]*memcache.Item
	addCalls map[string]*memcache.Item
	incrementCalls map[string]uint64
}

// NewMockMemcacheClient creates a new mock client.
func NewMockMemcacheClient() *mockMemcacheClient {
	return &mockMemcacheClient{
		setCalls: make(map[string]*memcache.Item),
		addCalls: make(map[string]*memcache.Item),
		incrementCalls: make(map[string]uint64),
	}
}

func (m *mockMemcacheClient) Get(key string) (*memcache.Item, error) {
	if m.GetFunc != nil {
		return m.GetFunc(key)
	}
	return nil, memcache.ErrCacheMiss 
}

func (m *mockMemcacheClient) Set(item *memcache.Item) error {
	if m.SetFunc != nil {
		return m.SetFunc(item)
	}
	m.setCalls[item.Key] = item 
	return nil                  
}

func (m *mockMemcacheClient) Add(item *memcache.Item) error {
	if m.AddFunc != nil {
		return m.AddFunc(item)
	}
	if _, exists := m.setCalls[item.Key]; exists { // Simple check, can be more sophisticated
		return memcache.ErrNotStored
	}
	m.addCalls[item.Key] = item
	m.setCalls[item.Key] = item // Add implies set for future Gets in this simple mock
	return nil
}

func (m *mockMemcacheClient) Increment(key string, delta uint64) (uint64, error) {
	if m.IncrementFunc != nil {
		return m.IncrementFunc(key, delta)
	}
	// This mock increment is very basic, assumes string "0" if not found.
	// Real memcache handles non-numeric values with errors.
	val, ok := m.setCalls[key]
	if !ok { // Or if Get(key) would return ErrCacheMiss
		m.setCalls[key] = &memcache.Item{Key:key, Value:[]byte(fmt.Sprintf("%d", delta))}
		return delta, nil
	}
	
	currentVal, err := strconv.ParseUint(string(val.Value), 10, 64)
	if err != nil {
		return 0, errors.New("memcache: value is not a number")
	}
	newVal := currentVal + delta
	m.setCalls[key].Value = []byte(fmt.Sprintf("%d", newVal))
	return newVal, nil
}


func (m *mockMemcacheClient) ResetSetCalls() {
	m.setCalls = make(map[string]*memcache.Item)
}

// --- TestNewLimiter ---
func TestNewLimiter_TokenBucketMemcache(t *testing.T) {
	mockClient := NewMockMemcacheClient()
	key := "test_tb_new"
	rate := 10
	capacity := 5

	// Test with default clock
	limiterDefault := tbmemcache.NewLimiter(key, rate, capacity, mockClient)
	if limiterDefault == nil {
		t.Fatal("NewLimiter with default clock returned nil")
	}

	// Test with injected clock
	limiterWithClock := tbmemcache.NewLimiter(key, rate, capacity, mockClient, tbmemcache.WithClock(mockNowFunc()))
	if limiterWithClock == nil {
		t.Fatal("NewLimiter with injected clock returned nil")
	}
	_, ok := limiterWithClock.(types.Limiter)
	if !ok {
		t.Fatalf("NewLimiter did not return a types.Limiter")
	}
}

// --- TestAllow ---
func TestAllow_TokenBucketMemcache(t *testing.T) {
	ctx := context.Background()
	limiterKey := "test_allow_tb"
	rate := 10
	capacity := 5
	identifier := "user1"
	expectedMemcacheKey := fmt.Sprintf("token_bucket:%s:%s", limiterKey, identifier)

	t.Run("SuccessfulAllowance_NewBucket", func(t *testing.T) {
		mockClient := NewMockMemcacheClient()
		limiter := tbmemcache.NewLimiter(limiterKey, rate, capacity, mockClient, tbmemcache.WithClock(mockNowFunc()))

		mockClient.GetFunc = func(key string) (*memcache.Item, error) {
			if key == expectedMemcacheKey {
				return nil, memcache.ErrCacheMiss
			}
			t.Fatalf("Unexpected Get key: %s", key)
			return nil, errors.New("unexpected key")
		}

		allowed, err := limiter.Allow(ctx, identifier)
		if err != nil {
			t.Fatalf("Allow failed: %v", err)
		}
		if !allowed {
			t.Fatal("Request unexpectedly denied for new bucket")
		}

		// Check if Set was called correctly
		setItem, ok := mockClient.setCalls[expectedMemcacheKey]
		if !ok {
			t.Fatalf("Expected Set to be called for key %s", expectedMemcacheKey)
		}
		var setState tokenBucketStateForTest 
		if err := json.Unmarshal(setItem.Value, &setState); err != nil {
			t.Fatalf("Failed to unmarshal Set item value: %v", err)
		}
		if setState.Tokens != int64(capacity-1) {
			t.Errorf("Expected tokens after first allow to be %d, got %d", capacity-1, setState.Tokens)
		}
		if !setState.LastRefill.Equal(mockTime) {
			t.Errorf("Expected LastRefill to be %v, got %v", mockTime, setState.LastRefill)
		}
	})

	t.Run("SuccessfulAllowance_ExistingBucket_SufficientTokens", func(t *testing.T) {
		mockClient := NewMockMemcacheClient()
		limiter := tbmemcache.NewLimiter(limiterKey, rate, capacity, mockClient, tbmemcache.WithClock(mockNowFunc()))

		// Initial state: 3 tokens, last refill was some time ago to allow full refill
		initialTokens := int64(3)
		initialLastRefill := mockTime.Add(-10 * time.Second) // Enough time ago for full refill
		initialData, _ := json.Marshal(tokenBucketStateForTest{Tokens: initialTokens, LastRefill: initialLastRefill})

		mockClient.GetFunc = func(key string) (*memcache.Item, error) {
			if key == expectedMemcacheKey {
				return &memcache.Item{Key: key, Value: initialData}, nil
			}
			return nil, memcache.ErrCacheMiss
		}

		allowed, err := limiter.Allow(ctx, identifier)
		if err != nil {
			t.Fatalf("Allow failed: %v", err)
		}
		if !allowed {
			t.Fatal("Request unexpectedly denied")
		}

		setItem := mockClient.setCalls[expectedMemcacheKey]
		var setState tokenBucketStateForTest
		json.Unmarshal(setItem.Value, &setState)
		// Tokens should be capacity - 1 because it refilled fully then one was consumed
		if setState.Tokens != int64(capacity-1) {
			t.Errorf("Expected tokens to be %d, got %d", capacity-1, setState.Tokens)
		}
	})
	
	t.Run("SuccessfulDenial_NoTokens", func(t *testing.T) {
		mockClient := NewMockMemcacheClient()
		// Set rate to 0 for this test to prevent refill and simplify token exhaustion
		limiter := tbmemcache.NewLimiter(limiterKey, 0, capacity, mockClient, tbmemcache.WithClock(mockNowFunc()))

		// Initial state: 0 tokens, last refill is current mockTime (no refill possible)
		initialTokens := int64(0)
		initialLastRefill := mockTime 
		initialData, _ := json.Marshal(tokenBucketStateForTest{Tokens: initialTokens, LastRefill: initialLastRefill})

		mockClient.GetFunc = func(key string) (*memcache.Item, error) {
			return &memcache.Item{Key: key, Value: initialData}, nil
		}
		
		allowed, err := limiter.Allow(ctx, identifier)
		if err != nil {
			t.Fatalf("Allow failed: %v", err)
		}
		if allowed {
			t.Fatal("Request unexpectedly allowed when no tokens")
		}
		setItem := mockClient.setCalls[expectedMemcacheKey]
		var setState tokenBucketStateForTest
		json.Unmarshal(setItem.Value, &setState)
		if setState.Tokens != 0 {
			t.Errorf("Expected tokens to remain 0, got %d", setState.Tokens)
		}
	})

	t.Run("TokenRefill", func(t *testing.T) {
		mockClient := NewMockMemcacheClient()
		// Use a specific clock for this test to advance time
		currentTime := mockTime
		mockNowFn := func() time.Time { return currentTime }
		
		limiter := tbmemcache.NewLimiter(limiterKey, rate, capacity, mockClient, tbmemcache.WithClock(mockNowFn))

		// Initial state: 0 tokens, last refill is current mockTime
		initialState := tokenBucketStateForTest{Tokens: 0, LastRefill: currentTime}
		// initialData, _ := json.Marshal(initialState) // This line was unused, remove it
		
		mockClient.GetFunc = func(key string) (*memcache.Item, error) {
			// Return a copy to avoid race if GetFunc is called multiple times by reserializing
			// For this test, the state is updated via prevSetState, so GetFunc can directly use initialState
			// as it's modified by the test logic itself.
			currentStateData, _ := json.Marshal(initialState) 
			return &memcache.Item{Key: key, Value: currentStateData}, nil
		}
		
		// 1. First attempt, no tokens, should be denied
		allowed, err := limiter.Allow(ctx, identifier)
		if err != nil { t.Fatalf("Allow 1 failed: %v", err) }
		if allowed { t.Fatal("Allow 1: Request unexpectedly allowed") }
		
		// Advance time by 1 second (should add 'rate' tokens, up to capacity)
		currentTime = currentTime.Add(1 * time.Second)
		
		// Update the state that GetFunc will return based on the Set call from previous Allow
		var prevSetState tokenBucketStateForTest
		json.Unmarshal(mockClient.setCalls[expectedMemcacheKey].Value, &prevSetState)
		initialState = prevSetState 

		// 2. Second attempt, after time advance, tokens should have refilled
		allowed, err = limiter.Allow(ctx, identifier)
		if err != nil { t.Fatalf("Allow 2 failed: %v", err) }
		if !allowed { t.Fatal("Allow 2: Request unexpectedly denied after refill") }

		setItem := mockClient.setCalls[expectedMemcacheKey]
		var setState tokenBucketStateForTest
		json.Unmarshal(setItem.Value, &setState)
		
		expectedTokens := int64(rate) 
		if expectedTokens > int64(capacity) { expectedTokens = int64(capacity) }
		expectedTokens-- 
		if setState.Tokens != expectedTokens {
			t.Errorf("Expected tokens after refill to be %d, got %d", expectedTokens, setState.Tokens)
		}
	})

	t.Run("MemcacheGetError", func(t *testing.T) {
		mockClient := NewMockMemcacheClient()
		limiter := tbmemcache.NewLimiter(limiterKey, rate, capacity, mockClient, tbmemcache.WithClock(mockNowFunc()))
		getErr := errors.New("memcache Get error")

		mockClient.GetFunc = func(key string) (*memcache.Item, error) {
			return nil, getErr
		}

		_, err := limiter.Allow(ctx, identifier)
		if err == nil {
			t.Fatal("Expected an error from Get but got nil")
		}
		if !strings.Contains(err.Error(), getErr.Error()) {
			t.Fatalf("Expected error containing '%s', got '%v'", getErr.Error(), err)
		}
	})

	t.Run("MemcacheSetError", func(t *testing.T) {
		mockClient := NewMockMemcacheClient()
		limiter := tbmemcache.NewLimiter(limiterKey, rate, capacity, mockClient, tbmemcache.WithClock(mockNowFunc()))
		setErr := errors.New("memcache Set error")

		mockClient.GetFunc = func(key string) (*memcache.Item, error) {
			return nil, memcache.ErrCacheMiss // Simulate new bucket
		}
		mockClient.SetFunc = func(item *memcache.Item) error {
			return setErr
		}

		_, err := limiter.Allow(ctx, identifier)
		if err == nil {
			t.Fatal("Expected an error from Set but got nil")
		}
		if !strings.Contains(err.Error(), setErr.Error()) {
			t.Fatalf("Expected error containing '%s', got '%v'", setErr.Error(), err)
		}
	})

	t.Run("JSONUnmarshalError", func(t *testing.T) {
		mockClient := NewMockMemcacheClient()
		limiter := tbmemcache.NewLimiter(limiterKey, rate, capacity, mockClient, tbmemcache.WithClock(mockNowFunc()))

		mockClient.GetFunc = func(key string) (*memcache.Item, error) {
			return &memcache.Item{Key: key, Value: []byte("invalid json")}, nil
		}

		_, err := limiter.Allow(ctx, identifier)
		if err == nil {
			t.Fatal("Expected an error from JSON unmarshal but got nil")
		}
		if !strings.Contains(err.Error(), "unmarshal state") { 
			t.Fatalf("Expected error containing 'unmarshal state', got '%v'", err)
		}
	})
}

// tokenBucketStateForTest is a local copy for testing, as the original is unexported.
type tokenBucketStateForTest struct {
    Tokens     int64     `json:"tokens"`
    LastRefill time.Time `json:"last_refill"`
}
