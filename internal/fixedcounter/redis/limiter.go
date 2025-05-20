// Package fcredis provides a Redis implementation of the Fixed Window Counter rate limiting algorithm.
package fcredis

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/rs/zerolog/log" // Import zerolog's global logger
)

// Limiter implements the Fixed Window Counter algorithm using Redis.
// It uses Redis keys to store the count for each window and Lua scripts for atomic operations.
type Limiter struct {
	client *redis.Client
	key    string // Limiter key from config
	window time.Duration
	limit  int64
	script *redis.Script
}

// NewLimiter creates a new Redis-based Fixed Window Counter limiter.
// It takes a Redis client instance, a unique key for the limiter, the size of the window, and the maximum limit of requests within the window.
func NewLimiter(client *redis.Client, key string, window time.Duration, limit int64) *Limiter {
	log.Info().Str("limiter_type", "FixedWindowCounter").Str("backend", "Redis").Str("limiter_key", key).Dur("window", window).Int64("limit", limit).Msg("Limiter: Initialized")
	return &Limiter{
		client: client,
		key:    key, // Store the key
		window: window,
		limit:  limit,
		script: redisAllowScript,
	}
}

// Allow checks if a request for the given identifier is allowed using a Redis Lua script.
// It takes a context and an identifier and returns true if the request is allowed, false otherwise, and an error if any occurred.
func (l *Limiter) Allow(ctx context.Context, identifier string) (bool, error) {
	redisKey := l.key + ":" + identifier

	nowMillis := time.Now().UnixMilli()
	windowMillis := l.window.Milliseconds()
	expirySeconds := int64(l.window.Seconds()) // Use window duration for expiry

	// Ensure expiry is at least 1 second if window is very short
	if expirySeconds < 1 {
		expirySeconds = 1
	}

	result, err := l.script.Run(ctx, l.client, []string{redisKey}, nowMillis, windowMillis, l.limit, expirySeconds).Result()
	if err != nil {
		// Added limiter key and identifier to error log
		log.Error().Err(err).Str("limiter_type", "FixedWindowCounter").Str("backend", "Redis").Str("limiter_key", l.key).Str("identifier", identifier).Str("redis_key", redisKey).Msg("Limiter: Redis script execution failed")
		return false, fmt.Errorf("redis script execution failed for limiter '%s', identifier '%s': %w", l.key, identifier, err)
	}

	allowed, ok := result.(int64)
	if !ok {
		err := fmt.Errorf("unexpected script result type: %T for key '%s', identifier '%s'", result, l.key, identifier)
		// Added limiter key and identifier to error log
		log.Error().Err(err).Str("limiter_type", "FixedWindowCounter").Str("backend", "Redis").Str("limiter_key", l.key).Str("identifier", identifier).Str("redis_key", redisKey).Type("result_type", result).Msg("Limiter: Unexpected script result type")
		return false, err
	}

	isAllowed := allowed == 1

	return isAllowed, nil
}
