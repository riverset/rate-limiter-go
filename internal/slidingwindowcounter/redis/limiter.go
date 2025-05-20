package swredis

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
)

type limiter struct {
	key        string
	client     *redis.Client
	windowSize time.Duration
	limit      int64
	script     *redis.Script
}

func NewLimiter(key string, windowSize time.Duration, limit int64, client *redis.Client) *limiter {
	log.Printf("Initialized redis Sliding Window Counter limiter for key '%s' with window %s and limit %d", key, windowSize, limit)
	return &limiter{
		key:        key,
		windowSize: windowSize,
		limit:      limit,
		client:     client,
		script:     redisAllowScript,
	}
}

func (l *limiter) Allow(ctx context.Context, identifier string) (bool, error) {
	// Construct the specific key for this identifier
	itemKey := l.key + ":" + identifier

	// Get current time in milliseconds
	now := time.Now().UnixNano() / int64(time.Millisecond)

	// Window size in milliseconds
	windowSizeMillis := l.windowSize.Milliseconds()

	// Execute the Lua script
	// KEYS: [itemKey]
	// ARGV: [now, windowSizeMillis, limit]
	result, err := l.script.Run(ctx, l.client, []string{itemKey}, now, windowSizeMillis, l.limit).Result()

	if err != nil {
		log.Printf("Error executing Redis script for key '%s': %v", itemKey, err)
		return false, err // Deny in case of error
	}

	// The script returns 1 for allowed, 0 for denied
	allowed, ok := result.(int64)
	if !ok {
		err := fmt.Errorf("unexpected result type from Redis script for key '%s': %T", itemKey, result)
		log.Printf("Unexpected result type from Redis script for key '%s': %T", itemKey, result)
		return false, err // Deny if result is not int64
	}

	return allowed == 1, nil
}
