// Package tbmemcache provides a Memcache implementation of the Token Bucket rate limiting algorithm.
package tbmemcache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/rs/zerolog/log"

	"learn.ratelimiter/internal/memcacheiface" // Use the new interface
	"learn.ratelimiter/types"
)

// limiter is the Memcache implementation of the Token Bucket.
type limiter struct {
	key      string
	capacity int
	rate     int
	client   memcacheiface.Client // Use the interface
	nowFunc  func() time.Time
}

// tokenBucketState represents the state of a token bucket stored in Memcache.
type tokenBucketState struct {
	Tokens     int64     `json:"tokens"`
	LastRefill time.Time `json:"last_refill"`
}

// NewLimiterOption is a function type for setting options on a Limiter.
type NewLimiterOption func(*limiter)

// WithClock sets a custom clock (nowFunc) for the Limiter.
func WithClock(nowFunc func() time.Time) NewLimiterOption {
	return func(l *limiter) {
		l.nowFunc = nowFunc
	}
}

// NewLimiter creates a new Memcache Token Bucket limiter.
func NewLimiter(key string, rate, capacity int, client memcacheiface.Client, opts ...NewLimiterOption) types.Limiter {
	limiterObj := &limiter{
		key:      key,
		rate:     rate,
		capacity: capacity,
		client:   client,
		nowFunc:  time.Now, // Default clock
	}
	for _, opt := range opts {
		opt(limiterObj)
	}
	log.Info().Str("limiter_type", "TokenBucket").Str("backend", "Memcache").Str("limiter_key", key).Int("rate", rate).Int("capacity", capacity).Msg("Limiter: Initialized")
	return limiterObj
}

// Allow checks if a request for the given identifier is allowed based on the Token Bucket algorithm.
func (l *limiter) Allow(ctx context.Context, identifier string) (bool, error) {
	itemKey := fmt.Sprintf("token_bucket:%s:%s", l.key, identifier)

	// Get the current state from Memcache
	item, err := l.client.Get(itemKey)
	if err != nil && err != memcache.ErrCacheMiss {
		log.Error().Err(err).Str("limiter_type", "TokenBucket").Str("backend", "Memcache").Str("limiter_key", l.key).Str("identifier", identifier).Msg("Limiter: Failed to get state from Memcache")
		return false, fmt.Errorf("get state from memcache: %w", err)
	}

	now := l.nowFunc() // Use injected clock
	state := &tokenBucketState{
		Tokens:     int64(l.capacity),
		LastRefill: now, // Initialize with current time from clock
	}

	if item != nil { // State exists in Memcache
		if err := json.Unmarshal(item.Value, state); err != nil {
			log.Error().Err(err).Str("limiter_type", "TokenBucket").Str("backend", "Memcache").Str("limiter_key", l.key).Str("identifier", identifier).Msg("Limiter: Failed to unmarshal state from Memcache")
			return false, fmt.Errorf("unmarshal state: %w", err)
		}
		// Refill tokens based on time elapsed since lastRefill from stored state
		elapsed := now.Sub(state.LastRefill)
		if elapsed.Seconds() > 0 { // Ensure time has actually passed to prevent negative refills if clock is manipulated weirdly
			refillAmount := int64(float64(l.rate) * elapsed.Seconds())
			state.Tokens += refillAmount
			if state.Tokens > int64(l.capacity) {
				state.Tokens = int64(l.capacity)
			}
		}
	}
	// For a new item (cache miss), state.Tokens is already l.capacity, LastRefill is 'now'.
	// No refill calculation needed for a brand new bucket.

	state.LastRefill = now // Always update lastRefill to current time

	// Check if allowed
	if state.Tokens >= 1 {
		state.Tokens--
		// Save the updated state back to Memcache
		value, err := json.Marshal(state)
		if err != nil {
			log.Error().Err(err).Str("limiter_type", "TokenBucket").Str("backend", "Memcache").Str("limiter_key", l.key).Str("identifier", identifier).Msg("Limiter: Failed to marshal state for Memcache")
			return false, fmt.Errorf("marshal state: %w", err)
		}
		if err := l.client.Set(&memcache.Item{Key: itemKey, Value: value}); err != nil {
			log.Error().Err(err).Str("limiter_type", "TokenBucket").Str("backend", "Memcache").Str("limiter_key", l.key).Str("identifier", identifier).Msg("Limiter: Failed to set state in Memcache")
			return false, fmt.Errorf("set state in memcache: %w", err)
		}
		log.Debug().Str("limiter_type", "TokenBucket").Str("backend", "Memcache").Str("limiter_key", l.key).Str("identifier", identifier).Int64("tokens", state.Tokens).Msg("Limiter: Request allowed")
		return true, nil
	} else {
		// Save the state even if denied to update lastRefill time
		value, err := json.Marshal(state)
		if err != nil {
			log.Error().Err(err).Str("limiter_type", "TokenBucket").Str("backend", "Memcache").Str("limiter_key", l.key).Str("identifier", identifier).Msg("Limiter: Failed to marshal state for Memcache")
			return false, fmt.Errorf("marshal state: %w", err)
		}
		if err := l.client.Set(&memcache.Item{Key: itemKey, Value: value}); err != nil {
			log.Error().Err(err).Str("limiter_type", "TokenBucket").Str("backend", "Memcache").Str("limiter_key", l.key).Str("identifier", identifier).Msg("Limiter: Failed to set state in Memcache")
			return false, fmt.Errorf("set state in memcache: %w", err)
		}
		log.Debug().Str("limiter_type", "TokenBucket").Str("backend", "Memcache").Str("limiter_key", l.key).Str("identifier", identifier).Int64("tokens", state.Tokens).Msg("Limiter: Request denied")
		return false, nil
	}
}
