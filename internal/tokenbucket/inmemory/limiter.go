package tbinmemory

import (
	"context"
	"log"
	"math"
	"sync"
	"time"
)

type limiter struct {
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

func NewLimiter(rate, capacity int) *limiter {
	log.Printf("Initialized in-memory Token Bucket limiter with rate %d and capacity %d", rate, capacity)
	return &limiter{
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
		log.Printf("Creating new token bucket for identifier '%s'", identifier)
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
		bucket.tokens = min(bucket.capacity, bucket.tokens+numTokensAdded)
		bucket.lastRefill = now // Update last refill time
	}

	// Check if context is cancelled before proceeding
	select {
	case <-ctx.Done():
		log.Printf("Context cancelled for identifier '%s' during token bucket check: %v", identifier, ctx.Err())
		return false, ctx.Err()
	default:
		// Continue
	}

	if bucket.tokens > 0 {
		bucket.tokens -= 1
		return true, nil
	}

	return false, nil
}
