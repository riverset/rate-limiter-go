package swredis

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
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
	log.Printf("SlidingWindowCounter(Redis): Initialized limiter '%s' with window %s and limit %d", key, windowSize, limit)
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
	// log.Printf("SlidingWindowCounter(Redis): Checking limiter '%s' for identifier '%s' (Redis key: %s)", l.key, identifier, redisKey) // Optional: verbose log

	result, err := l.script.Run(ctx, l.client, []string{redisKey}, now, windowSizeMillis, l.limit).Result()

	if err != nil {
		// Added limiter key and identifier to error log
		log.Printf("SlidingWindowCounter(Redis): Limiter '%s': Error executing script for identifier '%s' (Redis key: %s): %v", l.key, identifier, redisKey, err)
		return false, fmt.Errorf("redis script error for limiter '%s', identifier '%s': %w", l.key, identifier, err) // Deny in case of error
	}

	// The script returns 1 for allowed, 0 for denied
	allowed, ok := result.(int64)
	if !ok {
		err := fmt.Errorf("unexpected result type from Redis script for key '%s': %T", redisKey, result)
		// Added limiter key and identifier to error log
		log.Printf("SlidingWindowCounter(Redis): Limiter '%s': Unexpected result type from script for identifier '%s' (Redis key: %s): %T", l.key, identifier, redisKey, result)
		return false, err // Deny if result is not int64
	}

	isAllowed := allowed == 1
	// Optional: log allowed/denied status
	// log.Printf("SlidingWindowCounter(Redis): Limiter '%s': Identifier '%s' %s", l.key, identifier, map[bool]string{true: "allowed", false: "denied"}[isAllowed])

	return isAllowed, nil
}
