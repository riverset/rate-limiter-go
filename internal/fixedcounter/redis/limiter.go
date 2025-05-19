package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// Limiter implements the Fixed Window Counter algorithm using Redis.
type Limiter struct {
	client *redis.Client
	key    string // Base key for this limiter instance
	window time.Duration
	limit  int64
	script *redis.Script
}

// NewLimiter creates a new Redis-backed Fixed Window Counter limiter.
func NewLimiter(client *redis.Client, key string, window time.Duration, limit int64) *Limiter {
	return &Limiter{
		client: client,
		key:    key,
		window: window,
		limit:  limit,
		script: redisAllowScript, // Use the preloaded script
	}
}

// Allow checks if a request for the given identifier is allowed using a Redis Lua script.
func (l *Limiter) Allow(identifier string) (bool, error) {
	ctx := context.Background() // Use a proper context in a real application

	// The Redis key for this specific counter instance + identifier
	redisKey := l.key + ":" + identifier

	// Arguments for the Lua script:
	// ARGV[1]: current timestamp in milliseconds
	// ARGV[2]: window duration in milliseconds
	// ARGV[3]: limit
	// ARGV[4]: expiry time for the key (window duration in seconds)
	nowMillis := time.Now().UnixNano() / int64(time.Millisecond)
	windowMillis := l.window.Milliseconds()
	expirySeconds := int64(l.window.Seconds()) // Use seconds for Redis TTL

	// Execute the Lua script
	// KEYS: {redisKey}
	// ARGV: {nowMillis, windowMillis, l.limit, expirySeconds}
	result, err := l.script.Run(ctx, l.client, []string{redisKey}, nowMillis, windowMillis, l.limit, expirySeconds).Result()
	if err != nil {
		// Handle specific Redis errors if necessary
		return false, fmt.Errorf("redis script execution failed: %w", err)
	}

	// The script returns 1 if allowed, 0 if denied.
	allowed, ok := result.(int64)
	if !ok {
		return false, fmt.Errorf("unexpected script result type: %T", result)
	}

	return allowed == 1, nil
}
