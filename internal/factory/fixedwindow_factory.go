package factory

import (
	"fmt"

	"github.com/rs/zerolog/log" // Import zerolog's global logger

	"learn.ratelimiter/config"
	inmemoryfc "learn.ratelimiter/internal/fixedcounter/inmemory"
	redisfc "learn.ratelimiter/internal/fixedcounter/redis"
	"learn.ratelimiter/types"
)

// FixedWindowFactory creates limiters using the Fixed Window Counter algorithm.
type FixedWindowFactory struct{}

// NewFixedWindowFactory returns a new FixedWindowFactory instance.
func NewFixedWindowFactory() (*FixedWindowFactory, error) {
	return &FixedWindowFactory{}, nil
}

// CreateLimiter creates a Fixed Window Counter limiter based on the configuration and clients.
// It now returns types.Limiter and accepts types.BackendClients.
func (f *FixedWindowFactory) CreateLimiter(cfg config.LimiterConfig, clients types.BackendClients) (types.Limiter, error) {
	log.Info().Str("factory", "FixedWindowCounter").Str("limiter_key", cfg.Key).Str("backend", string(cfg.Backend)).Msg("Factory: Creating limiter")
	if cfg.WindowParams == nil {
		err := fmt.Errorf("fixed window counter parameters are missing in config for key '%s'", cfg.Key)
		log.Error().Err(err).Str("factory", "FixedWindowCounter").Str("limiter_key", cfg.Key).Msg("Factory: Creation failed")
		return nil, err
	}
	switch cfg.Backend {
	case config.InMemory:
		log.Info().Str("factory", "FixedWindowCounter").Str("backend", "InMemory").Str("limiter_key", cfg.Key).Dur("window", cfg.WindowParams.Window).Int64("limit", cfg.WindowParams.Limit).Msg("Factory: Creating in-memory limiter")
		return inmemoryfc.NewLimiter(cfg.Key, cfg.WindowParams.Window, cfg.WindowParams.Limit), nil
	case config.Redis:
		log.Info().Str("factory", "FixedWindowCounter").Str("backend", "Redis").Str("limiter_key", cfg.Key).Dur("window", cfg.WindowParams.Window).Int64("limit", cfg.WindowParams.Limit).Msg("Factory: Creating Redis limiter")
		if clients.RedisClient == nil {
			err := fmt.Errorf("redis client is required but not provided for redis backend for key '%s'", cfg.Key)
			log.Error().Err(err).Str("factory", "FixedWindowCounter").Str("backend", "Redis").Str("limiter_key", cfg.Key).Msg("Factory: Creation failed")
			return nil, err
		}
		return redisfc.NewLimiter(clients.RedisClient, cfg.Key, cfg.WindowParams.Window, cfg.WindowParams.Limit), nil
	case config.Memcache:
		err := fmt.Errorf("memcache backend not yet implemented for fixed window counter for key '%s'", cfg.Key)
		log.Error().Err(err).Str("factory", "FixedWindowCounter").Str("backend", "Memcache").Str("limiter_key", cfg.Key).Msg("Factory: Creation failed")
		return nil, err
	default:
		err := fmt.Errorf("unsupported backend type '%s' for fixed window counter for key '%s'", cfg.Backend, cfg.Key)
		log.Error().Err(err).Str("factory", "FixedWindowCounter").Str("backend", string(cfg.Backend)).Str("limiter_key", cfg.Key).Msg("Factory: Creation failed")
		return nil, err
	}
}
