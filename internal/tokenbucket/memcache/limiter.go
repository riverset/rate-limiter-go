// Package tbmemcache provides a Memcache implementation of the Token Bucket rate limiting algorithm.
package tbmemcache

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/rs/zerolog/log"

	"learn.ratelimiter/types"
)

// limiter is the Memcache implementation of the Token Bucket.
type limiter struct {
	key      string
	capacity int
	rate     int
	client   *memcache.Client
}

// tokenBucketState represents the state of a token bucket stored in Memcache.
type tokenBucketState struct {
	Tokens     int64     `json:"tokens"`
	LastRefill time.Time `json:"last_refill"`
}

// NewLimiter creates a new Memcache Token Bucket limiter.
func NewLimiter(key string, rate, capacity int, client *memcache.Client) types.Limiter {
	log.Info().Str("limiter_type", "TokenBucket").Str("backend", "Memcache").Str("limiter_key", key).Int("rate", rate).Int("capacity", capacity).Msg("Limiter: Initialized")
	return &limiter{
		key:      key,
		rate:     rate,
		capacity: capacity,
		client:   client,
	}
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

	state := &tokenBucketState{
		Tokens:     int64(l.capacity),
		LastRefill: time.Now(),
	}

	if item != nil {
		if err := json.Unmarshal(item.Value, state); err != nil {
			log.Error().Err(err).Str("limiter_type", "TokenBucket").Str("backend", "Memcache").Str("limiter_key", l.key).Str("identifier", identifier).Msg("Limiter: Failed to unmarshal state from Memcache")
			return false, fmt.Errorf("unmarshal state: %w", err)
		}
	}

	// Refill tokens
	now := time.Now()
	elapsed := now.Sub(state.LastRefill)
	refillAmount := int64(float64(l.rate) * elapsed.Seconds())
	state.Tokens = int64(math.Min(float64(state.Tokens)+float64(refillAmount), float64(l.capacity)))
	state.LastRefill = now

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
