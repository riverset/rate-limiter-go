package swcmemcache_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings" // Add missing import
	"testing"
	"time"

	"github.com/bradfitz/gomemcache/memcache"

	"learn.ratelimiter/internal/memcacheiface"
	swcmemcache "learn.ratelimiter/internal/slidingwindowcounter/memcache"
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

// mockMemcacheClient_SWC is a mock implementation for SlidingWindowCounter tests.
type mockMemcacheClient_SWC struct {
	memcacheiface.Client

	GetFunc func(key string) (*memcache.Item, error)
	SetFunc func(item *memcache.Item) error

	// Store calls for assertions
	getCalls map[string]int
	setCalls map[string]*memcache.Item
}

func NewMockMemcacheClient_SWC() *mockMemcacheClient_SWC {
	return &mockMemcacheClient_SWC{
		getCalls: make(map[string]int),
		setCalls: make(map[string]*memcache.Item),
	}
}

func (m *mockMemcacheClient_SWC) Get(key string) (*memcache.Item, error) {
	m.getCalls[key]++
	if m.GetFunc != nil {
		return m.GetFunc(key)
	}
	return nil, memcache.ErrCacheMiss // Default behavior
}

func (m *mockMemcacheClient_SWC) Set(item *memcache.Item) error {
	m.setCalls[item.Key] = item
	if m.SetFunc != nil {
		return m.SetFunc(item)
	}
	return nil // Default behavior
}

// Add and Increment are part of the interface but not used by SWC, so provide basic implementations.
func (m *mockMemcacheClient_SWC) Add(item *memcache.Item) error { return errors.New("not implemented for SWC mock") }
func (m *mockMemcacheClient_SWC) Increment(key string, delta uint64) (uint64, error) {
	return 0, errors.New("not implemented for SWC mock")
}

func (m *mockMemcacheClient_SWC) Reset() {
	m.getCalls = make(map[string]int)
	m.setCalls = make(map[string]*memcache.Item)
}


func TestNewLimiter_SlidingWindowMemcache(t *testing.T) {
	mockClient := NewMockMemcacheClient_SWC()
	keyPrefix := "test_sw_new"
	window := 60 * time.Second
	limit := 10

	limiterDefault := swcmemcache.NewLimiter(mockClient, keyPrefix, window, limit)
	if limiterDefault == nil {
		t.Fatal("NewLimiter (default clock) returned nil")
	}

	limiterWithClock := swcmemcache.NewLimiter(mockClient, keyPrefix, window, limit, swcmemcache.WithClock(mockNowFunc()))
	if limiterWithClock == nil {
		t.Fatal("NewLimiter (custom clock) returned nil")
	}
	_, ok := limiterWithClock.(types.Limiter)
	if !ok {
		t.Fatalf("NewLimiter did not return a types.Limiter")
	}
}


func TestAllow_SlidingWindowMemcache(t *testing.T) {
	ctx := context.Background()
	keyPrefix := "test_sw_allow"
	windowSize := 60 * time.Second // 60,000 ms
	limit := 3
	identifier := "user_sw_1"
	expectedMemcacheKey := fmt.Sprintf("%s:%s", keyPrefix, identifier)
	expectedExpiry := int32(windowSize.Seconds())


	t.Run("SuccessfulAllowance_EmptyState_FirstRequest", func(t *testing.T) {
		mockClient := NewMockMemcacheClient_SWC()
		limiter := swcmemcache.NewLimiter(mockClient, keyPrefix, windowSize, limit, swcmemcache.WithClock(mockNowFunc()))

		mockClient.GetFunc = func(key string) (*memcache.Item, error) {
			if key == expectedMemcacheKey {
				return nil, memcache.ErrCacheMiss
			}
			t.Fatalf("Get called with unexpected key: %s", key)
			return nil, errors.New("unexpected key")
		}

		allowed, err := limiter.Allow(ctx, identifier)
		if err != nil {
			t.Fatalf("Allow failed: %v", err)
		}
		if !allowed {
			t.Fatal("Request unexpectedly denied")
		}

		setItem, ok := mockClient.setCalls[expectedMemcacheKey]
		if !ok {
			t.Fatalf("Expected Set to be called for key %s", expectedMemcacheKey)
		}
		if setItem.Expiration != expectedExpiry {
			t.Errorf("Expected expiration %d, got %d", expectedExpiry, setItem.Expiration)
		}
		// For unit tests, we primarily verify that Set was called.
		// Deep inspection of the marshalled value is more for integration tests,
		// especially if the internal state struct is unexported.
		if setItem.Value == nil || len(setItem.Value) == 0 {
			t.Error("Set was called with nil or empty value")
		}
	})

	t.Run("SuccessfulAllowance_ExistingState_UnderLimit", func(t *testing.T) {
		mockClient := NewMockMemcacheClient_SWC()
		limiter := swcmemcache.NewLimiter(mockClient, keyPrefix, windowSize, limit, swcmemcache.WithClock(mockNowFunc()))

		// Existing state: 2 timestamps, both within the window
		existingTimestamps := []int64{
			mockTime.Add(-10 * time.Second).UnixMilli(), 
			mockTime.Add(-5 * time.Second).UnixMilli(),  
		}
		// Use the actual (potentially unexported) struct type from swcmemcache if possible,
		// or a test-local equivalent if not. For mocks, often just need valid JSON bytes.
		initialStateStruct := struct{ Timestamps []int64 }{Timestamps: existingTimestamps}
		initialData, _ := json.Marshal(initialStateStruct)
		mockClient.GetFunc = func(key string) (*memcache.Item, error) {
			return &memcache.Item{Key: key, Value: initialData}, nil
		}

		allowed, err := limiter.Allow(ctx, identifier)
		if err != nil {
			t.Fatalf("Allow failed: %v", err)
		}
		if !allowed {
			t.Fatal("Request unexpectedly denied")
		}

		setItem := mockClient.setCalls[expectedMemcacheKey]
		// As above, only check if Set was called with some data in unit test.
		if setItem.Value == nil || len(setItem.Value) == 0 {
			t.Error("Set was called with nil or empty value in ExistingState_UnderLimit")
		}
	})

	t.Run("SuccessfulAllowance_PruningOldTimestamps", func(t *testing.T) {
		mockClient := NewMockMemcacheClient_SWC()
		limiter := swcmemcache.NewLimiter(mockClient, keyPrefix, windowSize, limit, swcmemcache.WithClock(mockNowFunc()))
		
		// Existing state: 2 old timestamps, 1 recent
		existingTimestamps := []int64{
			mockTime.Add(-2 * windowSize).UnixMilli(),          
			mockTime.Add(-(windowSize + 5*time.Second)).UnixMilli(), 
			mockTime.Add(-5 * time.Second).UnixMilli(),          
		}
		initialStateStruct := struct{ Timestamps []int64 }{Timestamps: existingTimestamps}
		initialData, _ := json.Marshal(initialStateStruct)
		mockClient.GetFunc = func(key string) (*memcache.Item, error) {
			return &memcache.Item{Key: key, Value: initialData}, nil
		}

		allowed, err := limiter.Allow(ctx, identifier)
		if err != nil {
			t.Fatalf("Allow failed: %v", err)
		}
		if !allowed {
			t.Fatal("Request unexpectedly denied after pruning")
		}

		setItem := mockClient.setCalls[expectedMemcacheKey]
		// As above, only check if Set was called with some data in unit test.
		if setItem.Value == nil || len(setItem.Value) == 0 {
			t.Error("Set was called with nil or empty value in PruningOldTimestamps")
		}
	})
	
	t.Run("Denial_OverLimit", func(t *testing.T) {
		mockClient := NewMockMemcacheClient_SWC()
		limiter := swcmemcache.NewLimiter(mockClient, keyPrefix, windowSize, limit, swcmemcache.WithClock(mockNowFunc())) // limit is 3

		existingTimestamps := []int64{
			mockTime.Add(-10 * time.Second).UnixMilli(),
			mockTime.Add(-5 * time.Second).UnixMilli(),
			mockTime.Add(-2 * time.Second).UnixMilli(), 
		}
		initialStateStruct := struct{ Timestamps []int64 }{Timestamps: existingTimestamps}
		initialData, _ := json.Marshal(initialStateStruct)
		mockClient.GetFunc = func(key string) (*memcache.Item, error) {
			return &memcache.Item{Key: key, Value: initialData}, nil
		}

		allowed, err := limiter.Allow(ctx, identifier)
		if err != nil {
			t.Fatalf("Allow failed: %v", err)
		}
		if allowed {
			t.Fatal("Request unexpectedly allowed when over limit")
		}
		// Check if Set was called (implementation might choose not to save on denial, or save pruned)
		// Current implementation does not Set on denial.
		if _, ok := mockClient.setCalls[expectedMemcacheKey]; ok {
			t.Error("Set should not have been called on denial if state didn't change due to pruning (or if chosen not to save on denial)")
		}
	})

	t.Run("MemcacheGetError", func(t *testing.T) {
		mockClient := NewMockMemcacheClient_SWC()
		limiter := swcmemcache.NewLimiter(mockClient, keyPrefix, windowSize, limit, swcmemcache.WithClock(mockNowFunc()))
		expectedErr := errors.New("memcache Get error")
		mockClient.GetFunc = func(key string) (*memcache.Item, error) {
			return nil, expectedErr
		}

		_, err := limiter.Allow(ctx, identifier)
		if err == nil {
			t.Fatal("Expected Get error, got nil")
		}
		if !errors.Is(err, expectedErr) && !strings.Contains(err.Error(), expectedErr.Error()) {
			t.Fatalf("Expected error containing '%s', got '%v'", expectedErr.Error(), err)
		}
	})
	
	t.Run("MemcacheSetError_StillAllowsRequest", func(t *testing.T) {
		mockClient := NewMockMemcacheClient_SWC()
		limiter := swcmemcache.NewLimiter(mockClient, keyPrefix, windowSize, limit, swcmemcache.WithClock(mockNowFunc()))
		expectedSetErr := errors.New("memcache Set error")

		mockClient.GetFunc = func(key string) (*memcache.Item, error) {
			return nil, memcache.ErrCacheMiss // New item
		}
		mockClient.SetFunc = func(item *memcache.Item) error {
			return expectedSetErr
		}
		
		allowed, err := limiter.Allow(ctx, identifier)
		if err != nil { // The main error path might not return the Set error directly
			// Check if the log would have contained it. For test, we check if allowed.
			// The current implementation logs Set errors but still returns true.
			t.Logf("Allow returned error: %v (this might be ok if it's just a logged Set error)", err)
		}
		if !allowed {
			t.Fatal("Request should be allowed even if Set fails (as per current impl logic)")
		}
		// No direct error return for Set failure in Allow path, but it's logged.
	})

	t.Run("JSONUnmarshalError", func(t *testing.T) {
		mockClient := NewMockMemcacheClient_SWC()
		limiter := swcmemcache.NewLimiter(mockClient, keyPrefix, windowSize, limit, swcmemcache.WithClock(mockNowFunc()))
		mockClient.GetFunc = func(key string) (*memcache.Item, error) {
			return &memcache.Item{Key: key, Value: []byte("invalid json")}, nil
		}

		_, err := limiter.Allow(ctx, identifier)
		if err == nil {
			t.Fatal("Expected Unmarshal error, got nil")
		}
		if !strings.Contains(err.Error(), "memcache unmarshal failed") {
			t.Fatalf("Expected error containing 'memcache unmarshal failed', got '%v'", err)
		}
	})
}
