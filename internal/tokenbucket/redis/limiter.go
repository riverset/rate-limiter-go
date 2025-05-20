package tbredis

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/rs/zerolog/log" // Import zerolog's global logger

	"learn.ratelimiter/types"
)

type Limiter struct {
	key      string
	rate     int // tokens per second
	capacity int
	client   *redis.Client
	script   *redis.Script
}

func NewLimiter(key string, rate int, capacity int, client *redis.Client) types.Limiter {
	log.Info().Str("limiter_type", "TokenBucket").Str("backend", "Redis").Str("limiter_key", key).Int("rate", rate).Int("capacity", capacity).Msg("Limiter: Initialized")

	return &Limiter{
		key:      key,
		rate:     rate,
		capacity: capacity,
		client:   client,
		script:   redisAllowScript,
	}
}

func (l *Limiter) Allow(ctx context.Context, identifier string) (bool, error) {
	// The actual key in Redis will be a combination of the limiter key and the identifier
	redisKey := fmt.Sprintf("%s:%s", l.key, identifier)

	now := time.Now().UnixMilli()

	result, err := l.script.Run(
		ctx,
		l.client,
		[]string{redisKey},
		l.capacity,
		l.rate,
		now,
		1, // tokens to consume
	).Result()

	if err != nil {
		log.Error().Err(err).Str("limiter_type", "TokenBucket").Str("backend", "Redis").Str("limiter_key", l.key).Str("identifier", identifier).Str("redis_key", redisKey).Msg("Limiter: Redis script execution failed")
		return false, fmt.Errorf("redis script error for limiter '%s', identifier '%s': %w", l.key, identifier, err)
	}

	// The script returns a two-element array: [allowed, tokens]
	// allowed is 1 if the request is allowed, 0 otherwise
	// tokens is the number of tokens remaining after the request
	results, ok := result.([]interface{})
	if !ok || len(results) != 2 {
		log.Error().Str("limiter_type", "TokenBucket").Str("backend", "Redis").Str("limiter_key", l.key).Str("identifier", identifier).Str("redis_key", redisKey).Msg("Limiter: Unexpected result from redis script")
		return false, fmt.Errorf("unexpected result from redis script for limiter '%s', identifier '%s'", l.key, identifier)
	}

	allowed, ok := results[0].(int64)
	if !ok {
		log.Error().Str("limiter_type", "TokenBucket").Str("backend", "Redis").Str("limiter_key", l.key).Str("identifier", identifier).Str("redis_key", redisKey).Msg("Limiter: Unexpected allowed value type from redis script")
		return false, fmt.Errorf("unexpected allowed value type from redis script for limiter '%s', identifier '%s'", l.key, identifier)
	}

	return allowed == 1, nil
}
