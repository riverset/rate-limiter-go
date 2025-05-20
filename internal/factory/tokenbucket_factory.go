// Package factory provides factories for creating different rate limiter instances.
package factory

import (
	"fmt"

	"github.com/rs/zerolog/log" // Import zerolog's global logger

	"learn.ratelimiter/config"
	tbinmemory "learn.ratelimiter/internal/tokenbucket/inmemory"
	redistb "learn.ratelimiter/internal/tokenbucket/redis"
	"learn.ratelimiter/types"
)

// TokenBucketFactory creates limiters using the Token Bucket algorithm.
type TokenBucketFactory struct{}

// NewTokenBucketFactory returns a new TokenBucketFactory instance.
func NewTokenBucketFactory() (*TokenBucketFactory, error) {
	return &TokenBucketFactory{}, nil
}

// CreateLimiter creates a Token Bucket limiter based on the configuration and backend clients.
// It takes a LimiterConfig and BackendClients and returns a types.Limiter or an error.
func (*TokenBucketFactory) CreateLimiter(cfg config.LimiterConfig, clients types.BackendClients) (types.Limiter, error) {
	log.Info().Str("factory", "TokenBucket").Str("limiter_key", cfg.Key).Str("backend", string(cfg.Backend)).Msg("Factory: Creating limiter")
	if cfg.TokenBucketParams == nil {
		err := fmt.Errorf("token bucket parameters are missing in config for key '%s'", cfg.Key)
		log.Error().Err(err).Str("factory", "TokenBucket").Str("limiter_key", cfg.Key).Msg("Factory: Creation failed")
		return nil, err
	}

	switch cfg.Backend {
	case config.InMemory:
		// Added parameters to log
		log.Info().Str("factory", "TokenBucket").Str("backend", "InMemory").Str("limiter_key", cfg.Key).Int("rate", cfg.TokenBucketParams.Rate).Int("capacity", cfg.TokenBucketParams.Capacity).Msg("Factory: Creating in-memory limiter")
		// Assuming the inmemory package has a New function matching the signature
		return tbinmemory.NewLimiter(cfg.Key, cfg.TokenBucketParams.Rate, cfg.TokenBucketParams.Capacity), nil // Pass key to in-memory limiter
	case config.Redis:
		// Added parameters to log
		log.Info().Str("factory", "TokenBucket").Str("backend", "Redis").Str("limiter_key", cfg.Key).Int("rate", cfg.TokenBucketParams.Rate).Int("capacity", cfg.TokenBucketParams.Capacity).Msg("Factory: Creating Redis limiter")
		if clients.RedisClient == nil {
			err := fmt.Errorf("redis client is required but not provided for redis backend for key '%s'", cfg.Key)
			log.Error().Err(err).Str("factory", "TokenBucket").Str("backend", "Redis").Str("limiter_key", cfg.Key).Msg("Factory: Creation failed")
			return nil, err
		}
		return redistb.NewLimiter(cfg.Key, cfg.TokenBucketParams.Rate, cfg.TokenBucketParams.Capacity, clients.RedisClient), nil

	case config.Memcache:
		err := fmt.Errorf("memcache backend not yet implemented for token bucket for key '%s'", cfg.Key)
		log.Error().Err(err).Str("factory", "TokenBucket").Str("backend", "Memcache").Str("limiter_key", cfg.Key).Msg("Factory: Creation failed")
		return nil, err
	default:
		err := fmt.Errorf("unsupported backend type '%s' for token bucket for key '%s'", cfg.Backend, cfg.Key)
		log.Error().Err(err).Str("factory", "TokenBucket").Str("backend", string(cfg.Backend)).Str("limiter_key", cfg.Key).Msg("Factory: Creation failed")
		return nil, err
	}
}
