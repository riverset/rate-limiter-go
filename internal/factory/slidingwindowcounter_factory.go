// Package factory provides factories for creating different rate limiter instances.
package factory

import (
	"fmt"

	"github.com/rs/zerolog/log" // Import zerolog's global logger

	"learn.ratelimiter/config"
	swinmemory "learn.ratelimiter/internal/slidingwindowcounter/inmemory"
	swredis "learn.ratelimiter/internal/slidingwindowcounter/redis"
	"learn.ratelimiter/types"
)

// SlidingWindowCounterFactory creates limiters using the Sliding Window Counter algorithm.
type SlidingWindowCounterFactory struct{}

// NewSlidingWindowCounterFactory returns a new SlidingWindowCounterFactory instance.
func NewSlidingWindowCounterFactory() (*SlidingWindowCounterFactory, error) {
	return &SlidingWindowCounterFactory{}, nil
}

// CreateLimiter creates a Sliding Window Counter limiter based on the configuration and backend clients.
// It takes a LimiterConfig and BackendClients and returns a types.Limiter or an error.
func (*SlidingWindowCounterFactory) CreateLimiter(cfg config.LimiterConfig, clients types.BackendClients) (types.Limiter, error) {
	log.Info().Str("factory", "SlidingWindowCounter").Str("limiter_key", cfg.Key).Str("backend", string(cfg.Backend)).Msg("Factory: Creating limiter")
	if cfg.WindowParams == nil {
		err := fmt.Errorf("sliding window counter parameters are missing in config for key '%s'", cfg.Key)
		log.Error().Err(err).Str("factory", "SlidingWindowCounter").Str("limiter_key", cfg.Key).Msg("Factory: Creation failed")
		return nil, err
	}

	switch cfg.Backend {
	case config.InMemory:
		// Added parameters to log
		log.Info().Str("factory", "SlidingWindowCounter").Str("backend", "InMemory").Str("limiter_key", cfg.Key).Dur("window", cfg.WindowParams.Window).Int64("limit", cfg.WindowParams.Limit).Msg("Factory: Creating in-memory limiter")
		return swinmemory.NewLimiter(cfg.Key, cfg.WindowParams.Window, cfg.WindowParams.Limit), nil
	case config.Redis:
		// Added parameters to log
		log.Info().Str("factory", "SlidingWindowCounter").Str("backend", "Redis").Str("limiter_key", cfg.Key).Dur("window", cfg.WindowParams.Window).Int64("limit", cfg.WindowParams.Limit).Msg("Factory: Creating Redis limiter")
		if clients.RedisClient == nil {
			err := fmt.Errorf("redis client is required but not provided for redis backend for key '%s'", cfg.Key)
			log.Error().Err(err).Str("factory", "SlidingWindowCounter").Str("backend", "Redis").Str("limiter_key", cfg.Key).Msg("Factory: Creation failed")
			return nil, err
		}
		return swredis.NewLimiter(cfg.Key, cfg.WindowParams.Window, cfg.WindowParams.Limit, clients.RedisClient), nil

	case config.Memcache:
		err := fmt.Errorf("memcache backend not yet implemented for sliding window counter for key '%s'", cfg.Key)
		log.Error().Err(err).Str("factory", "SlidingWindowCounter").Str("backend", "Memcache").Str("limiter_key", cfg.Key).Msg("Factory: Creation failed")
		return nil, err
	default:
		err := fmt.Errorf("unsupported backend type '%s' for sliding window counter for key '%s'", cfg.Backend, cfg.Key)
		log.Error().Err(err).Str("factory", "SlidingWindowCounter").Str("backend", string(cfg.Backend)).Str("limiter_key", cfg.Key).Msg("Factory: Creation failed")
		return nil, err

	}
}
