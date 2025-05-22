// Package fcmemcache provides a Memcache implementation of the Fixed Window Counter rate limiting algorithm.
package fcmemcache

import (
	"context"
	"fmt"
	"time"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/rs/zerolog/log"

	"learn.ratelimiter/internal/memcacheiface" // Using the defined interface
	"learn.ratelimiter/types"
)

type Limiter struct {
	client     memcacheiface.Client
	keyPrefix  string
	window     time.Duration
	limit      int
	// nowFunc is not strictly needed for this fixed window version if expiry handles reset,
	// but kept for consistency if options pattern is used.
	nowFunc    func() time.Time 
}

// NewLimiterOption is a function type for setting options on a Limiter.
type NewLimiterOption func(*Limiter)

// WithClock sets a custom clock (nowFunc) for the Limiter.
// Note: nowFunc is not used by this specific fixed-window counter's Allow logic,
// as window reset is handled by Memcache key expiry. It's included for API consistency.
func WithClock(nowFunc func() time.Time) NewLimiterOption {
	return func(l *Limiter) {
		l.nowFunc = nowFunc
	}
}

func NewLimiter(client memcacheiface.Client, keyPrefix string, window time.Duration, limit int, opts ...NewLimiterOption) types.Limiter {
	l := &Limiter{
		client:    client,
		keyPrefix: keyPrefix,
		window:    window,
		limit:     limit,
		nowFunc:   time.Now, // Default, though not used in Allow's core logic here
	}
	for _, opt := range opts {
		opt(l)
	}
	log.Info().Str("limiter_type", "FixedWindowCounter").Str("backend", "Memcache").Str("limiter_key_prefix", keyPrefix).Dur("window", window).Int("limit", limit).Msg("Limiter: Initialized")
	return l
}

func (l *Limiter) Allow(ctx context.Context, identifier string) (bool, error) {
	memcacheKey := fmt.Sprintf("%s:%s", l.keyPrefix, identifier)
	expirySeconds := int32(l.window.Seconds())
	if expirySeconds < 1 {
		expirySeconds = 1 
	}

	// Try to Add the key with count "1". This sets it with expiry.
	// The value stored is string representation of the count.
	item := &memcache.Item{
		Key:        memcacheKey,
		Value:      []byte("1"),
		Expiration: expirySeconds,
	}
	err := l.client.Add(item)

	if err == nil { // Successfully added, count is 1.
		if 1 <= l.limit {
			log.Debug().Str("limiter", l.keyPrefix).Str("id", identifier).Int("count", 1).Msg("Allowed (added)")
			return true, nil
		}
		log.Debug().Str("limiter", l.keyPrefix).Str("id", identifier).Int("count", 1).Msg("Denied (added, over limit)")
		return false, nil
	}

	if err == memcache.ErrNotStored { // Key already exists.
		// Increment the existing key.
		// Increment does not update TTL. The original TTL from Add (or a previous Set) remains.
		// This is a key behavior: the window is defined by the first Add.
		newValue, incErr := l.client.Increment(memcacheKey, 1)
		if incErr != nil {
			// If Increment fails because the key disappeared between Add and Increment (very unlikely but possible),
			// or if value is not a number. For simplicity, we treat this as a general error.
			// A production system might retry Add or Get & Set.
			log.Error().Err(incErr).Str("limiter", l.keyPrefix).Str("id", identifier).Msg("Failed to increment counter")
			return false, fmt.Errorf("memcache increment failed: %w", incErr)
		}

		count := int(newValue) 
		if count <= l.limit {
			log.Debug().Str("limiter", l.keyPrefix).Str("id", identifier).Int("count", count).Msg("Allowed (incremented)")
			return true, nil
		}
		log.Debug().Str("limiter", l.keyPrefix).Str("id", identifier).Int("count", count).Msg("Denied (incremented, over limit)")
		return false, nil
	}

	// Any other error from Add.
	log.Error().Err(err).Str("limiter", l.keyPrefix).Str("id", identifier).Msg("Failed to add/increment counter")
	return false, fmt.Errorf("memcache Add operation failed: %w", err)
}
