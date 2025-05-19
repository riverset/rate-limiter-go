package factory

import (
	"fmt"

	"learn.ratelimiter/config"
	"learn.ratelimiter/core"
	inmemoryfc "learn.ratelimiter/internal/fixedcounter/inmemory"
	redisfc "learn.ratelimiter/internal/fixedcounter/redis"
)

// FixedWindowFactory creates limiters using the Fixed Window Counter algorithm.
type FixedWindowFactory struct{}

// NewFixedWindowFactory returns a new FixedWindowFactory instance.
func NewFixedWindowFactory() *FixedWindowFactory {
	return &FixedWindowFactory{}
}

// CreateLimiter creates a Fixed Window Counter limiter based on the configuration and clients.
func (f *FixedWindowFactory) CreateLimiter(cfg config.LimiterConfig, clients core.BackendClients) (core.Limiter, error) {
	if cfg.FixedWindowCounterParams == nil {
		return nil, fmt.Errorf("fixed window counter parameters are missing in config for key '%s'", cfg.Key)
	}
	switch cfg.Backend {
	case config.InMemory:
		return inmemoryfc.NewLimiter(cfg.Key, cfg.FixedWindowCounterParams.Window, cfg.FixedWindowCounterParams.Limit), nil
	case config.Redis:
		if clients.RedisClient == nil {
			return nil, fmt.Errorf("redis client is required but not provided for redis backend for key '%s'", cfg.Key)
		}
		return redisfc.NewLimiter(clients.RedisClient, cfg.Key, cfg.FixedWindowCounterParams.Window, cfg.FixedWindowCounterParams.Limit), nil
	case config.Memcache:
		return nil, fmt.Errorf("memcache backend not yet implemented for fixed window counter for key '%s'", cfg.Key)
	default:
		return nil, fmt.Errorf("unsupported backend type '%s' for fixed window counter for key '%s'", cfg.Backend, cfg.Key)
	}
}
