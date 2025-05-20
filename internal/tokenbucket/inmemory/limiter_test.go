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
