package tbinmemory

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/rs/zerolog/log" // Import zerolog's global logger
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
	log.Info().Str("limiter_type", "TokenBucket").Str("backend", "InMemory").Str("limiter_key", key).Int("rate", rate).Int("capacity", capacity).Msg("Limiter: Initialized")
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
		log.Debug().Str("limiter_type", "TokenBucket").Str("backend", "InMemory").Str("limiter_key", l.key).Str("identifier", identifier).Msg("Limiter: Creating new token bucket")
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
		// Added limiter key and identifier to log
		log.Warn().Err(ctx.Err()).Str("limiter_type", "TokenBucket").Str("backend", "InMemory").Str("limiter_key", l.key).Str("identifier", identifier).Msg("Limiter: Context cancelled during check")
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
