// Package lbinmemory provides an in-memory implementation of the Leaky Bucket rate limiting algorithm.
package lbinmemory

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"learn.ratelimiter/types"
)

// limiter is the in-memory implementation of the Leaky Bucket.
type limiter struct {
	key      string
	rate     int
	capacity int
	mu       sync.Mutex
	// currentLevel is the current number of tokens in the bucket.
	currentLevel float64
	// lastLeak is the last time tokens were leaked from the bucket.
	lastLeak time.Time
}

// NewLimiter creates a new in-memory Leaky Bucket limiter.
func NewLimiter(key string, rate, capacity int) types.Limiter {
	log.Info().Str("limiter_type", "LeakyBucket").Str("backend", "InMemory").Str("limiter_key", key).Int("rate", rate).Int("capacity", capacity).Msg("Limiter: Initialized")
	return &limiter{
		key:          key,
		rate:         rate,
		capacity:     capacity,
		currentLevel: 0,
		lastLeak:     time.Now(),
	}
}

// Allow checks if a request for the given identifier is allowed based on the Leaky Bucket algorithm.
func (l *limiter) Allow(ctx context.Context, identifier string) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(l.lastLeak)
	leakedAmount := elapsed.Seconds() * float64(l.rate)

	l.currentLevel = math.Max(0, l.currentLevel-leakedAmount)
	l.lastLeak = now

	if l.currentLevel+1 <= float64(l.capacity) {
		l.currentLevel++
		log.Debug().Str("limiter_type", "LeakyBucket").Str("backend", "InMemory").Str("limiter_key", l.key).Str("identifier", identifier).Float64("current_level", l.currentLevel).Msg("Limiter: Request allowed")
		return true, nil
	} else {
		log.Debug().Str("limiter_type", "LeakyBucket").Str("backend", "InMemory").Str("limiter_key", l.key).Str("identifier", identifier).Float64("current_level", l.currentLevel).Msg("Limiter: Request denied")
		return false, nil
	}
}
