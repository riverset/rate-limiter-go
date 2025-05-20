package swredis

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/rs/zerolog/log" // Import zerolog's global logger
)

type limiter struct {
	key        string // Limiter key from config
	client     *redis.Client
	windowSize time.Duration
	limit      int64
	script     *redis.Script
}

// Added key parameter to NewLimiter
func NewLimiter(key string, windowSize time.Duration, limit int64, client *redis.Client) *limiter {
	log.Info().Str("limiter_type", "SlidingWindowCounter").Str("backend", "Redis").Str("limiter_key", key).Dur("window", windowSize).Int64("limit", limit).Msg("Limiter: Initialized")
	return &limiter{
		key:        key, // Store the key
		windowSize: windowSize,
		limit:      limit,
		client:     client,
		script:     redisAllowScript,
	}
}

func (l *limiter) Allow(ctx context.Context, identifier string) (bool, error) {
	// Construct the specific key for this identifier
	redisKey := l.key + ":" + identifier

	// Get current time in milliseconds
	now := time.Now().UnixMilli()

	// Window size in milliseconds
	windowSizeMillis := l.windowSize.Milliseconds()

	// Execute the Lua script
	// KEYS: [itemKey]
	// ARGV: [now, windowSizeMillis, limit]

	result, err := l.script.Run(ctx, l.client, []string{redisKey}, now, windowSizeMillis, l.limit).Result()

	if err != nil {
		// Added limiter key and identifier to error log
		log.Error().Err(err).Str("limiter_type", "SlidingWindowCounter").Str("backend", "Redis").Str("limiter_key", l.key).Str("identifier", identifier).Str("redis_key", redisKey).Msg("Limiter: Error executing script")
		return false, fmt.Errorf("redis script error for limiter '%s', identifier '%s': %w", l.key, identifier, err) // Deny in case of error
	}

	// The script returns 1 for allowed, 0 for denied
	allowed, ok := result.(int64)
	if !ok {
		err := fmt.Errorf("unexpected result type from Redis script for key '%s': %T", redisKey, result)
		// Added limiter key and identifier to error log
		log.Error().Err(err).Str("limiter_type", "SlidingWindowCounter").Str("backend", "Redis").Str("limiter_key", l.key).Str("identifier", identifier).Str("redis_key", redisKey).Type("result_type", result).Msg("Limiter: Unexpected result type from script")
		return false, err // Deny if result is not int64
	}

	isAllowed := allowed == 1

	return isAllowed, nil
}
