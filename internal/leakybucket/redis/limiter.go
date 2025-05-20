// Package lbredis provides a Redis implementation of the Leaky Bucket rate limiting algorithm.
package lbredis

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/rs/zerolog/log"

	"learn.ratelimiter/types"
)

const leakyBucketLuaScript = `
-- KEYS[1]: The key for the bucket state (e.g., leaky_bucket:limiter_key:identifier)
-- ARGV[1]: Capacity of the bucket
-- ARGV[2]: Leak rate (tokens per second)
-- ARGV[3]: Current timestamp in milliseconds

local capacity = tonumber(ARGV[1])
local rate = tonumber(ARGV[2])
local now = tonumber(ARGV[3])

local res = redis.call('GET', KEYS[1])

local currentLevel = 0
local lastLeak = now

if res then
    local state = cjson.decode(res)
    currentLevel = tonumber(state['currentLevel'])
    lastLeak = tonumber(state['lastLeak'])
end

local elapsed = (now - lastLeak) / 1000 -- elapsed time in seconds
local leakedAmount = elapsed * rate

currentLevel = math.max(0, currentLevel - leakedAmount)

local allowed = false
if currentLevel + 1 <= capacity then
    currentLevel = currentLevel + 1
    allowed = true
end

lastLeak = now

local newState = cjson.encode({currentLevel = currentLevel, lastLeak = lastLeak})
redis.call('SET', KEYS[1], newState)

if allowed then
    return 1
else
    return 0
end
`

// limiter is the Redis implementation of the Leaky Bucket.
type limiter struct {
	key      string
	rate     int
	capacity int
	client   *redis.Client
	script   *redis.Script
}

// NewLimiter creates a new Redis Leaky Bucket limiter.
func NewLimiter(key string, rate, capacity int, client *redis.Client) types.Limiter {
	log.Info().Str("limiter_type", "LeakyBucket").Str("backend", "Redis").Str("limiter_key", key).Int("rate", rate).Int("capacity", capacity).Msg("Limiter: Initialized")
	script := redis.NewScript(leakyBucketLuaScript)
	return &limiter{
		key:      key,
		rate:     rate,
		capacity: capacity,
		client:   client,
		script:   script,
	}
}

// Allow checks if a request for the given identifier is allowed based on the Leaky Bucket algorithm.
func (l *limiter) Allow(ctx context.Context, identifier string) (bool, error) {
	itemKey := fmt.Sprintf("leaky_bucket:%s:%s", l.key, identifier)
	now := time.Now().UnixNano() / int64(time.Millisecond)

	result, err := l.script.Run(ctx, l.client, []string{itemKey}, l.capacity, l.rate, now).Result()
	if err != nil {
		log.Error().Err(err).Str("limiter_type", "LeakyBucket").Str("backend", "Redis").Str("limiter_key", l.key).Str("identifier", identifier).Msg("Limiter: Failed to run Lua script")
		return false, fmt.Errorf("run leaky bucket lua script: %w", err)
	}

	allowed, ok := result.(int64)
	if !ok {
		err := fmt.Errorf("unexpected result type from redis script: %T", result)
		log.Error().Err(err).Str("limiter_type", "LeakyBucket").Str("backend", "Redis").Str("limiter_key", l.key).Str("identifier", identifier).Msg("Limiter: Unexpected script result")
		return false, err
	}

	return allowed == 1, nil
}
