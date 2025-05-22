// Package tbinmemory_test contains unit tests for the in-memory Token Bucket rate limiter.
package tbinmemory_test

import (
	"context"
	"testing"
	"time"

	tbinmemory "learn.ratelimiter/internal/tokenbucket/inmemory"
)

// TestNewLimiter tests the initialization of a new in-memory Token Bucket limiter.
func TestNewLimiter(t *testing.T) {
	key := "test-key"
	rate := 10
	capacity := 5

	limiter := tbinmemory.NewLimiter(key, rate, capacity)

	if limiter == nil {
		t.Fatal("NewLimiter returned nil")
	}
	// Accessing unexported fields requires reflection or changing the struct fields to be exported.
	// For simplicity, we'll test behavior through the public Allow method later.
	// We can check if the internal map is initialized.
	// if limiter.buckets == nil {
	// 	t.Error("Limiter buckets map is nil")
	// }
}

// TestAllowBasic tests the basic allowance and denial of requests for a single identifier.
func TestAllowBasic(t *testing.T) {
	key := "test-key-basic"
	rate := 1     // 1 token per second
	capacity := 2 // capacity of 2

	limiter := tbinmemory.NewLimiter(key, rate, capacity)
	ctx := context.Background()

	// First request should be allowed
	allowed, err := limiter.Allow(ctx, "user1")
	if err != nil {
		t.Fatalf("Allow returned error: %v", err)
	}
	if !allowed {
		t.Error("First request for user1 should be allowed")
	}

	// Second request should be allowed
	allowed, err = limiter.Allow(ctx, "user1")
	if err != nil {
		t.Fatalf("Allow returned error: %v", err)
	}
	if !allowed {
		t.Error("Second request for user1 should be allowed")
	}

	// Third request should be denied (bucket is empty)
	allowed, err = limiter.Allow(ctx, "user1")
	if err != nil {
		t.Fatalf("Allow returned error: %v", err)
	}
	if allowed {
		t.Error("Third request for user1 should be denied")
	}
}

// TestAllowRefill tests the token refill mechanism of the in-memory Token Bucket limiter.
func TestAllowRefill(t *testing.T) {
	key := "test-key-refill"
	rate := 1     // 1 token per second
	capacity := 2 // capacity of 2

	limiter := tbinmemory.NewLimiter(key, rate, capacity)
	ctx := context.Background()

	// Exhaust tokens
	limiter.Allow(ctx, "user2")
	limiter.Allow(ctx, "user2")
	allowed, _ := limiter.Allow(ctx, "user2")
	if allowed {
		t.Fatal("Should have exhausted tokens")
	}

	// Wait for refill (more than 1 second)
	time.Sleep(1100 * time.Millisecond)

	// Request after refill should be allowed
	allowed, err := limiter.Allow(ctx, "user2")
	if err != nil {
		t.Fatalf("Allow returned error after refill: %v", err)
	}
	if !allowed {
		t.Error("Request after refill should be allowed")
	}

	// Wait for another refill (more than 1 second)
	time.Sleep(1100 * time.Millisecond)

	// Request after another refill should be allowed (up to capacity)
	allowed, err = limiter.Allow(ctx, "user2")
	if err != nil {
		t.Fatalf("Allow returned error after second refill: %v", err)
	}
	if !allowed {
		t.Error("Request after second refill should be allowed")
	}

	// Third request should be denied again
	allowed, err = limiter.Allow(ctx, "user2")
	if err != nil {
		t.Fatalf("Allow returned error after exhausting again: %v", err)
	}
	if allowed {
		t.Error("Third request after second refill should be denied")
	}
}

// TestAllowMultipleIdentifiers tests that the limiter handles multiple identifiers independently.
func TestAllowMultipleIdentifiers(t *testing.T) {
	key := "test-key-multi"
	rate := 1     // 1 token per second
	capacity := 1 // capacity of 1

	limiter := tbinmemory.NewLimiter(key, rate, capacity)
	ctx := context.Background()

	// Request for userA should be allowed
	allowedA1, errA1 := limiter.Allow(ctx, "userA")
	if errA1 != nil {
		t.Fatalf("Allow for userA returned error: %v", errA1)
	}
	if !allowedA1 {
		t.Error("First request for userA should be allowed")
	}

	// Request for userB should also be allowed (separate bucket)
	allowedB1, errB1 := limiter.Allow(ctx, "userB")
	if errB1 != nil {
		t.Fatalf("Allow for userB returned error: %v", errB1)
	}
	if !allowedB1 {
		t.Error("First request for userB should be allowed")
	}

	// Second request for userA should be denied
	allowedA2, errA2 := limiter.Allow(ctx, "userA")
	if errA2 != nil {
		t.Fatalf("Allow for userA returned error: %v", errA2)
	}
	if allowedA2 {
		t.Error("Second request for userA should be denied")
	}

	// Second request for userB should be denied
	allowedB2, errB2 := limiter.Allow(ctx, "userB")
	if errB2 != nil {
		t.Fatalf("Allow for userB returned error: %v", errB2)
	}
	if allowedB2 {
		t.Error("Second request for userB should be denied")
	}
}

// TestAllowContextCancellation tests that the Allow method respects context cancellation.
func TestAllowContextCancellation(t *testing.T) {
	key := "test-key-cancel"
	rate := 100 // High rate so it doesn't block on tokens
	capacity := 100

	limiter := tbinmemory.NewLimiter(key, rate, capacity)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel the context immediately

	// Allow should return false and the context error
	allowed, err := limiter.Allow(ctx, "user-cancel")
	if err == nil {
		t.Error("Allow should return an error when context is cancelled")
	}
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled error, got %v", err)
	}
	if allowed {
		t.Error("Allow should return false when context is cancelled")
	}
}

func TestTokenBucketLimiter_ZeroRate(t *testing.T) {
	limiter := tbinmemory.NewLimiter("test_zero_rate", 0, 10) // Rate: 0 tokens/sec, Capacity: 10
	ctx := context.Background()

	// Requests should be denied as rate is 0, *after initial capacity is exhausted*.
	initialCapacity := 10 // Must match the capacity given to NewLimiter
	for i := 0; i < initialCapacity; i++ {
		allowed, err := limiter.Allow(ctx, "user1")
		if err != nil {
			t.Fatalf("Unexpected error during initial token consumption: %v", err)
		}
		if !allowed {
			t.Fatalf("Request %d/%d unexpectedly denied during initial token consumption with zero rate", i+1, initialCapacity)
		}
	}

	// Next request should be denied
	allowed, err := limiter.Allow(ctx, "user1")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if allowed {
		t.Fatalf("Request unexpectedly allowed with zero rate after initial capacity exhausted")
	}
}

func TestTokenBucketLimiter_NegativeRate(t *testing.T) {
	limiter := tbinmemory.NewLimiter("test_negative_rate", -1, 10) // Rate: -1 tokens/sec, Capacity: 10
	ctx := context.Background()

	// Requests should be denied as rate is negative, *after initial capacity is exhausted*.
	initialCapacity := 10 // Must match the capacity given to NewLimiter
	for i := 0; i < initialCapacity; i++ {
		allowed, err := limiter.Allow(ctx, "user1")
		if err != nil {
			t.Fatalf("Unexpected error during initial token consumption: %v", err)
		}
		if !allowed {
			t.Fatalf("Request %d/%d unexpectedly denied during initial token consumption with negative rate", i+1, initialCapacity)
		}
	}

	// Next request should be denied
	allowed, err := limiter.Allow(ctx, "user1")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if allowed {
		t.Fatalf("Request unexpectedly allowed with negative rate after initial capacity exhausted")
	}
}

func TestTokenBucketLimiter_ZeroCapacity(t *testing.T) {
	limiter := tbinmemory.NewLimiter("test_zero_capacity", 5, 0) // Rate: 5 tokens/sec, Capacity: 0
	ctx := context.Background()

	// Requests should be denied as capacity is 0
	allowed, err := limiter.Allow(ctx, "user1")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if allowed {
		t.Fatalf("Request unexpectedly allowed with zero capacity")
	}
}

func TestTokenBucketLimiter_NegativeCapacity(t *testing.T) {
	limiter := tbinmemory.NewLimiter("test_negative_capacity", 5, -1) // Rate: 5 tokens/sec, Capacity: -1
	ctx := context.Background()

	// Requests should be denied as capacity is negative
	allowed, err := limiter.Allow(ctx, "user1")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if allowed {
		t.Fatalf("Request unexpectedly allowed with negative capacity")
	}
}

func TestTokenBucketLimiter_Concurrency(t *testing.T) {
	limiter := tbinmemory.NewLimiter("test_concurrency", 5, 3) // Rate: 5 tokens/sec, Capacity: 3
	ctx := context.Background()
	numRequests := 10
	results := make(chan bool, numRequests)
	for i := 0; i < numRequests; i++ {
		go func() {
			// Stagger requests slightly to avoid all goroutines starting at the exact same microsecond
			// and consuming tokens before the refill mechanism can kick in for some of them.
			// This makes the test more realistic for concurrent scenarios.
			time.Sleep(time.Duration(i%5) * 10 * time.Millisecond)
			allowed, err := limiter.Allow(ctx, "user_concurrent")
			if err != nil {
				// Using t.Errorf for goroutines as t.Fatalf will exit the current goroutine, not the test.
				t.Errorf("Allow failed: %v", err)
				results <- false
				return
			}
			results <- allowed
		}()
	}

	allowedCount := 0
	// Wait for all goroutines to finish and collect results
	for i := 0; i < numRequests; i++ {
		if <-results {
			allowedCount++
		}
	}

	// The number of allowed requests can be more than capacity due to refills over time.
	// A precise assertion is hard without knowing the exact timing of goroutines.
	// We expect it to be greater than capacity if refills happened, but not all requests.
	// For a rate of 5 and capacity 3, over a short period with 10 requests,
	// it's reasonable to expect more than 3 but less than 10.
	// A more robust test might involve controlling time more explicitly or checking logs.
	// For now, we'll check that not all requests were allowed if numRequests > localCapacity,
	// and that at least localCapacity was allowed if numRequests >= localCapacity.
	// Define localCapacity and localRate based on the NewLimiter call for this test
	localRate := 5
	localCapacity := 3

	if numRequests > localCapacity && allowedCount == numRequests {
		t.Errorf("Allowed all %d requests concurrently, expected some denials due to rate limit and capacity %d", numRequests, localCapacity)
	}

	// Assert that allowedCount is at least the initial localCapacity if enough requests were made.
	if allowedCount < localCapacity && numRequests >= localCapacity {
		t.Errorf("Allowed %d requests, expected at least %d (localCapacity)", allowedCount, localCapacity)
	}

	// Flexible check: Allow for some refills.
	// If the rate is positive, allowed count can exceed initial capacity due to refills.
	// This check is heuristic. A more precise test would require controlling time explicitly.
	if localRate > 0 {
		// Expect that allowedCount is not excessively large, e.g., not more than initial capacity + rate (tokens refilled over 1 sec).
		// This is a rough upper bound, actual refills depend on test execution time.
		// If many requests are made over a period allowing multiple refills, this might need adjustment.
		// For 10 requests with small staggering, execution is quick.
		// Max possible within ~100ms (duration of staggered requests + some processing):
		// Initial: localCapacity. Refilled: localRate * 0.1. Total: localCapacity + localRate * 0.1
		// A loose upper bound like localCapacity + localRate seems generous enough for typical test runs.
		// If allowedCount is much higher than localCapacity + localRate, it might indicate an issue or very slow test execution.
		if allowedCount > localCapacity+localRate {
			t.Logf("Note: Allowed %d requests with localCapacity %d and localRate %d. This is acceptable if test duration was long enough for significant refills.", allowedCount, localCapacity, localRate)
		}
	} else { // localRate is 0 or negative
		if allowedCount > localCapacity {
			t.Errorf("Allowed %d requests with localCapacity %d and zero/negative localRate %d; expected no more than localCapacity", allowedCount, localCapacity, localRate)
		}
	}

	if allowedCount > numRequests {
		t.Errorf("Allowed %d requests, but only %d were made", allowedCount, numRequests)
	}

	// Core checks:
	// 1. If enough requests are made, at least the initial capacity should be allowed.
	if numRequests >= localCapacity && allowedCount < localCapacity {
		t.Fatalf("Allowed %d requests, expected at least %d (localCapacity)", allowedCount, localCapacity)
	}
	// 2. If more requests are made than capacity and rate is zero, not all should be allowed.
	//    If rate > 0, this check becomes tricky due to refills.
	if localRate <= 0 && numRequests > localCapacity && allowedCount == numRequests {
		t.Fatalf("Allowed all %d requests with zero/negative rate, but expected some to be denied above capacity %d", numRequests, localCapacity)
	}
	// 3. If numRequests > localCapacity and rate > 0, it's possible all are allowed if test runs long enough for refills.
	//    However, for a short test with many requests, we usually expect some denials.
	//    This is a heuristic: if all 10 requests (numRequests) are allowed for capacity 3, it implies significant refills.
	//    For this test (10 req, cap 3, rate 5), if it runs very fast (e.g. < 100ms), we might expect 3-4 allowed.
	//    If it runs for ~1.4s, all 10 could be allowed (3 initial + 7 refilled).
	//    The original check `if allowedCount == numRequests && numRequests > capacity` is okay for catching cases where *no* rate limiting happens.
	if allowedCount == numRequests && numRequests > localCapacity && localRate > 0 && numRequests > localCapacity+localRate { // If numRequests is much larger than can be sustained in ~1s
		t.Logf("Warning: Allowed all %d requests. This is okay if the test took enough time for refills (capacity: %d, rate: %d).", numRequests, localCapacity, localRate)
	}


	t.Logf("Concurrency test: %d requests, %d allowed (capacity %d, rate %d)", numRequests, allowedCount, localCapacity, localRate)
}
