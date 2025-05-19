package redis

import (
	"context"
	"fmt"
	"log" // Added log import
	"time"

	"github.com/go-redis/redis/v8"
)

// Limiter implements the Fixed Window Counter algorithm using Redis.
type Limiter struct {
	client *redis.Client
	key    string
	window time.Duration
	limit  int64
	script *redis.Script
}

// NewLimiter creates a new Redis-backed Fixed Window Counter limiter.
func NewLimiter(client *redis.Client, key string, window time.Duration, limit int64) *Limiter {
	log.Printf("Initialized Redis Fixed Window Counter limiter for key '%s' with window %s and limit %d", key, window, limit)
	return &Limiter{
		client: client,
		key:    key,
		window: window,
		limit:  limit,
		script: redisAllowScript,
	}
}

// Allow checks if a request for the given identifier is allowed using a Redis Lua script.
// It now accepts a context.Context parameter and passes it to the Redis client.
func (l *Limiter) Allow(ctx context.Context, identifier string) (bool, error) {
	redisKey := l.key + ":" + identifier

	nowMillis := time.Now().UnixNano() / int64(time.Millisecond)
	windowMillis := l.window.Milliseconds()
	expirySeconds := int64(l.window.Seconds())

	result, err := l.script.Run(ctx, l.client, []string{redisKey}, nowMillis, windowMillis, l.limit, expirySeconds).Result()
	if err != nil {
		log.Printf("Redis script execution failed for key '%s', identifier '%s': %v", l.key, identifier, err)
		return false, fmt.Errorf("redis script execution failed for key '%s', identifier '%s': %w", l.key, identifier, err)
	}

	allowed, ok := result.(int64)
	if !ok {
		err := fmt.Errorf("unexpected script result type: %T for key '%s', identifier '%s'", result, l.key, identifier)
		log.Printf("Error in Allow for key '%s', identifier '%s': %v", l.key, identifier, err)
		return false, err
	}

	isAllowed := allowed == 1

	return isAllowed, nil
}
