package factory

import (
	"fmt"
	"log"

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
	log.Printf("Factory(FixedWindowCounter): Creating limiter for key '%s' with backend '%s'", cfg.Key, cfg.Backend)
	if cfg.WindowParams == nil {
		err := fmt.Errorf("fixed window counter parameters are missing in config for key '%s'", cfg.Key)
		log.Printf("Factory(FixedWindowCounter): Creation failed for key '%s': %v", cfg.Key, err)
		return nil, err
	}
	switch cfg.Backend {
	case config.InMemory:
		log.Printf("Factory(FixedWindowCounter): Creating in-memory limiter for key '%s' with window %s and limit %d", cfg.Key, cfg.WindowParams.Window, cfg.WindowParams.Limit)
		return inmemoryfc.NewLimiter(cfg.Key, cfg.WindowParams.Window, cfg.WindowParams.Limit), nil
	case config.Redis:
		log.Printf("Factory(FixedWindowCounter): Creating Redis limiter for key '%s' with window %s and limit %d", cfg.Key, cfg.WindowParams.Window, cfg.WindowParams.Limit)
		if clients.RedisClient == nil {
			err := fmt.Errorf("redis client is required but not provided for redis backend for key '%s'", cfg.Key)
			log.Printf("Factory(FixedWindowCounter): Creation failed for key '%s': %v", cfg.Key, err)
			return nil, err
		}
		return redisfc.NewLimiter(clients.RedisClient, cfg.Key, cfg.WindowParams.Window, cfg.WindowParams.Limit), nil
	case config.Memcache:
		err := fmt.Errorf("memcache backend not yet implemented for fixed window counter for key '%s'", cfg.Key)
		log.Printf("Factory(FixedWindowCounter): Creation failed for key '%s': %v", cfg.Key, err)
		return nil, err
	default:
		err := fmt.Errorf("unsupported backend type '%s' for fixed window counter for key '%s'", cfg.Backend, cfg.Key)
		log.Printf("Factory(FixedWindowCounter): Creation failed for key '%s': %v", cfg.Key, err)
		return nil, err
	}
}
