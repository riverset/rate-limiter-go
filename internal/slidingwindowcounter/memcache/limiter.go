// Package swcmemcache provides a Memcache implementation of the Sliding Window Counter rate limiting algorithm.
package swcmemcache

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/rs/zerolog/log"

	"learn.ratelimiter/internal/memcacheiface"
	"learn.ratelimiter/types"
)

type Limiter struct {
	client     memcacheiface.Client
	keyPrefix  string
	windowSize time.Duration
	limit      int
	nowFunc    func() time.Time
}

// requestTimestamps stores the timestamps of requests for an identifier.
type requestTimestamps struct {
	Timestamps []int64 `json:"timestamps"`
}

// NewLimiterOption is a function type for setting options on a Limiter.
type NewLimiterOption func(*Limiter)

// WithClock sets a custom clock (nowFunc) for the Limiter.
func WithClock(nowFunc func() time.Time) NewLimiterOption {
	return func(l *Limiter) {
		l.nowFunc = nowFunc
	}
}

func NewLimiter(client memcacheiface.Client, keyPrefix string, windowSize time.Duration, limit int, opts ...NewLimiterOption) types.Limiter {
	l := &Limiter{
		client:     client,
		keyPrefix:  keyPrefix,
		windowSize: windowSize,
		limit:      limit,
		nowFunc:    time.Now,
	}
	for _, opt := range opts {
		opt(l)
	}
	log.Info().Str("limiter_type", "SlidingWindowCounter").Str("backend", "Memcache").Str("limiter_key_prefix", keyPrefix).Dur("window", windowSize).Int("limit", limit).Msg("Limiter: Initialized")
	return l
}

func (l *Limiter) Allow(ctx context.Context, identifier string) (bool, error) {
	memcacheKey := fmt.Sprintf("%s:%s", l.keyPrefix, identifier)
	now := l.nowFunc()
	nowMillis := now.UnixMilli()
	windowStartMillis := nowMillis - l.windowSize.Milliseconds()
	expirySeconds := int32(l.windowSize.Seconds())
	if expirySeconds < 1 {
		expirySeconds = 1
	}

	item, err := l.client.Get(memcacheKey)
	if err != nil && err != memcache.ErrCacheMiss {
		log.Error().Err(err).Str("limiter", l.keyPrefix).Str("id", identifier).Msg("Failed to get timestamps from Memcache")
		return false, fmt.Errorf("memcache get failed: %w", err)
	}

	var state requestTimestamps
	if err == memcache.ErrCacheMiss {
		// No existing timestamps, this is the first request in a while or new identifier
		state.Timestamps = []int64{}
	} else { // Key found
		if errUnmarshal := json.Unmarshal(item.Value, &state); errUnmarshal != nil {
			log.Error().Err(errUnmarshal).Str("limiter", l.keyPrefix).Str("id", identifier).Msg("Failed to unmarshal timestamps from Memcache")
			// Treat as corrupted data? Or allow based on empty? For safety, deny and log.
			return false, fmt.Errorf("memcache unmarshal failed: %w", errUnmarshal)
		}
	}

	// Prune old timestamps
	validTimestamps := []int64{}
	for _, ts := range state.Timestamps {
		if ts >= windowStartMillis {
			validTimestamps = append(validTimestamps, ts)
		}
	}
	sort.Slice(validTimestamps, func(i, j int) bool { return validTimestamps[i] < validTimestamps[j] }) // Keep sorted

	if len(validTimestamps) < l.limit {
		validTimestamps = append(validTimestamps, nowMillis)
		newState := requestTimestamps{Timestamps: validTimestamps}
		newValue, errMarshal := json.Marshal(newState)
		if errMarshal != nil {
			log.Error().Err(errMarshal).Str("limiter", l.keyPrefix).Str("id", identifier).Msg("Failed to marshal new timestamps")
			return false, fmt.Errorf("json marshal failed: %w", errMarshal)
		}

		setItem := &memcache.Item{
			Key:        memcacheKey,
			Value:      newValue,
			Expiration: expirySeconds,
		}
		if errSet := l.client.Set(setItem); errSet != nil {
			log.Error().Err(errSet).Str("limiter", l.keyPrefix).Str("id", identifier).Msg("Failed to set new timestamps in Memcache")
			// Allow request even if set fails? Or deny? For this impl, we'll assume if set fails, we can't guarantee limits.
			// However, the request was technically allowed before the set.
			// This is tricky. A common approach is to allow but log the error.
			// For stricter adherence, one might deny if persistence of the new count fails.
			// Let's log and allow, as the check passed.
			log.Warn().Err(errSet).Str("limiter", l.keyPrefix).Str("id", identifier).Msg("Allowed, but failed to save updated timestamps to Memcache")
		}
		log.Debug().Str("limiter", l.keyPrefix).Str("id", identifier).Int("count", len(validTimestamps)).Msg("Allowed")
		return true, nil
	}

	// Denied, over limit
	// Optionally, save the pruned list back to memcache to clean up old entries
	// For this implementation, we won't save on denial to reduce writes,
	// but this means old timestamps only get pruned when a request *would have been* allowed.
	log.Debug().Str("limiter", l.keyPrefix).Str("id", identifier).Int("count", len(validTimestamps)).Msg("Denied (over limit)")
	return false, nil
}
