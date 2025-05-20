package fcredis

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
)

// Limiter implements the Fixed Window Counter algorithm using Redis.
type Limiter struct {
	client *redis.Client
	key    string // Limiter key from config
	window time.Duration
	limit  int64
	script *redis.Script
}

// Added key parameter to NewLimiter
func NewLimiter(client *redis.Client, key string, window time.Duration, limit int64) *Limiter {
	log.Printf("FixedWindowCounter(Redis): Initialized limiter '%s' with window %s and limit %d", key, window, limit)
	return &Limiter{
		client: client,
		key:    key, // Store the key
		window: window,
		limit:  limit,
		script: redisAllowScript,
	}
}

// Allow checks if a request for the given identifier is allowed using a Redis Lua script.
// It now accepts a context.Context parameter and passes it to the Redis client.
func (l *Limiter) Allow(ctx context.Context, identifier string) (bool, error) {
	redisKey := l.key + ":" + identifier

	nowMillis := time.Now().UnixMilli()
	windowMillis := l.window.Milliseconds()
	expirySeconds := int64(l.window.Seconds()) // Use window duration for expiry

	// Ensure expiry is at least 1 second if window is very short
	if expirySeconds < 1 {
		expirySeconds = 1
	}

	// log.Printf("FixedWindowCounter(Redis): Checking limiter '%s' for identifier '%s' (Redis key: %s)", l.key, identifier, redisKey) // Optional: verbose log

	result, err := l.script.Run(ctx, l.client, []string{redisKey}, nowMillis, windowMillis, l.limit, expirySeconds).Result()
	if err != nil {
		// Added limiter key and identifier to error log
		log.Printf("FixedWindowCounter(Redis): Limiter '%s': Script execution failed for identifier '%s' (Redis key: %s): %v", l.key, identifier, redisKey, err)
		return false, fmt.Errorf("redis script execution failed for limiter '%s', identifier '%s': %w", l.key, identifier, err)
	}

	allowed, ok := result.(int64)
	if !ok {
		err := fmt.Errorf("unexpected script result type: %T for key '%s', identifier '%s'", result, l.key, identifier)
		// Added limiter key and identifier to error log
		log.Printf("FixedWindowCounter(Redis): Limiter '%s': Unexpected script result type for identifier '%s' (Redis key: %s): %T", l.key, identifier, redisKey, result)
		return false, err
	}

	isAllowed := allowed == 1
	// Optional: log allowed/denied status
	// log.Printf("FixedWindowCounter(Redis): Limiter '%s': Identifier '%s' %s", l.key, identifier, map[bool]string{true: "allowed", false: "denied"}[isAllowed])

	return isAllowed, nil
}
