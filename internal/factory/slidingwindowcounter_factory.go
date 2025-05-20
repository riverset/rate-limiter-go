package factory

import (
	"fmt"
	"log"

	"learn.ratelimiter/config"
	"learn.ratelimiter/internal/fixedcounter/inmemory"
	"learn.ratelimiter/types"
)

type SlidingWindowCounterFactory struct{}

func NewSlidingWindowCounterFactory() (*SlidingWindowCounterFactory, error) {
	return &SlidingWindowCounterFactory{}, nil
}
func (*SlidingWindowCounterFactory) CreateLimiter(cfg config.LimiterConfig, clients types.BackendClients) (types.Limiter, error) {
	if cfg.WindowParams == nil {
		err := fmt.Errorf("sliding window counter parameters are missing in config for key '%s'", cfg.Key)
		log.Printf("Creation failed: %v", err)
		return nil, err
	}

	switch cfg.Backend {
	case config.InMemory:
		log.Printf("Creating in-memory Sliding Window Counter limiter for key '%s'", cfg.Key)
		return inmemory.NewLimiter(cfg.Key, cfg.WindowParams.Window, cfg.WindowParams.Limit), nil
	case config.Redis:
		err := fmt.Errorf("redis backend not yet implemented for sliding window counter for key '%s'", cfg.Key)
		log.Printf("Creation failed: %v", err)
		return nil, err

	case config.Memcache:
		err := fmt.Errorf("memcache backend not yet implemented for sliding window counter for key '%s'", cfg.Key)
		log.Printf("Creation failed: %v", err)
		return nil, err
	default:
		err := fmt.Errorf("unsupported backend type '%s' for sliding window counter for key '%s'", cfg.Backend, cfg.Key)
		log.Printf("Creation failed: %v", err)
		return nil, err

	}
}
