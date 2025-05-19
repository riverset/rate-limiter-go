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
	key    string
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
		script: redisAllowScript,
	}
}

// Allow checks if a request for the given identifier is allowed using a Redis Lua script.
func (l *Limiter) Allow(identifier string) (bool, error) {
	ctx := context.Background()

	redisKey := l.key + ":" + identifier

	nowMillis := time.Now().UnixNano() / int64(time.Millisecond)
	windowMillis := l.window.Milliseconds()
	expirySeconds := int64(l.window.Seconds())

	result, err := l.script.Run(ctx, l.client, []string{redisKey}, nowMillis, windowMillis, l.limit, expirySeconds).Result()
	if err != nil {
		return false, fmt.Errorf("redis script execution failed: %w", err)
	}

	allowed, ok := result.(int64)
	if !ok {
		return false, fmt.Errorf("unexpected script result type: %T", result)
	}

	return allowed == 1, nil
}
