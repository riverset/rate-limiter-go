package tbinmemory

import (
	"context"
	"log"
	"math"
	"sync"
	"time"
)

type limiter struct {
	key      string // Limiter key from config
	buckets  map[string]*tokenBucket
	rate     int
	capacity int
	mu       sync.Mutex
}

type tokenBucket struct {
	tokens     int
	capacity   int
	lastRefill time.Time
}

// Added key parameter to NewLimiter
func NewLimiter(key string, rate, capacity int) *limiter {
	log.Printf("TokenBucket(InMemory): Initialized limiter '%s' with rate %d and capacity %d", key, rate, capacity)
	return &limiter{
		key:      key, // Store the key
		buckets:  make(map[string]*tokenBucket),
		rate:     rate,
		capacity: capacity,
	}
}

// Allow checks if a request for the given identifier is allowed.
// Updated to match core.Limiter interface.
func (l *limiter) Allow(ctx context.Context, identifier string) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	bucket, exists := l.buckets[identifier]
	if !exists {
		// Added limiter key and identifier to log
		log.Printf("TokenBucket(InMemory): Limiter '%s': Creating new token bucket for identifier '%s'", l.key, identifier)
		l.buckets[identifier] = &tokenBucket{
			tokens:     l.capacity,
			capacity:   l.capacity,
			lastRefill: time.Now(),
		}
		bucket = l.buckets[identifier]
	}

	// Refill tokens
	now := time.Now()
	numTokensAdded := int(math.Floor(now.Sub(bucket.lastRefill).Seconds() * float64(l.rate)))
	if numTokensAdded > 0 {
		// log.Printf("TokenBucket(InMemory): Limiter '%s': Identifier '%s': Refilled %d tokens. Old tokens: %d, New tokens: %d", l.key, identifier, numTokensAdded, bucket.tokens, min(bucket.capacity, bucket.tokens+numTokensAdded)) // Optional: verbose log
		bucket.tokens = min(bucket.capacity, bucket.tokens+numTokensAdded)
		bucket.lastRefill = now // Update last refill time
	}

	// Check if context is cancelled before proceeding
	select {
	case <-ctx.Done():
		// Added limiter key and identifier to log
		log.Printf("TokenBucket(InMemory): Limiter '%s': Context cancelled for identifier '%s' during check: %v", l.key, identifier, ctx.Err())
		return false, ctx.Err()
	default:
		// Continue
	}

	// log.Printf("TokenBucket(InMemory): Limiter '%s': Identifier '%s': Current tokens %d, Requested 1", l.key, identifier, bucket.tokens) // Optional: verbose log

	if bucket.tokens > 0 {
		bucket.tokens -= 1
		// log.Printf("TokenBucket(InMemory): Limiter '%s': Identifier '%s' allowed. Remaining tokens: %d", l.key, identifier, bucket.tokens) // Optional: verbose log
		return true, nil
	}

	// log.Printf("TokenBucket(InMemory): Limiter '%s': Identifier '%s' denied. No tokens left.", l.key, identifier) // Optional: verbose log
	return false, nil
}
