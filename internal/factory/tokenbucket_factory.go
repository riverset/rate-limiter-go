package factory

import (
	"fmt"
	"log"

	"learn.ratelimiter/config"
	tbinmemory "learn.ratelimiter/internal/tokenbucket/inmemory"
	redistb "learn.ratelimiter/internal/tokenbucket/redis"
	"learn.ratelimiter/types"
)

type TokenBucketFactory struct{}

func NewTokenBucketFactory() (*TokenBucketFactory, error) {
	return &TokenBucketFactory{}, nil
}

func (*TokenBucketFactory) CreateLimiter(cfg config.LimiterConfig, clients types.BackendClients) (types.Limiter, error) {
	if cfg.TokenBucketParams == nil {
		err := fmt.Errorf("token bucket parameters are missing in config for key '%s'", cfg.Key)
		log.Printf("Creation failed: %v", err)
		return nil, err
	}

	switch cfg.Backend {
	case config.InMemory:
		log.Printf("Creating in-memory Token Bucket limiter for key '%s'", cfg.Key)
		// Assuming the inmemory package has a New function matching the signature
		return tbinmemory.NewLimiter(cfg.TokenBucketParams.Rate, cfg.TokenBucketParams.Capacity), nil
	case config.Redis:
		log.Printf("Creating Redis Token Bucket limiter for key '%s'", cfg.Key)
		if clients.RedisClient == nil {
			err := fmt.Errorf("redis client is required but not provided for redis backend for key '%s'", cfg.Key)
			log.Printf("Creation failed: %v", err)
			return nil, err
		}
		return redistb.NewLimiter(cfg.Key, cfg.TokenBucketParams.Rate, cfg.TokenBucketParams.Capacity, clients.RedisClient), nil

	case config.Memcache:
		err := fmt.Errorf("memcache backend not yet implemented for token bucket for key '%s'", cfg.Key)
		log.Printf("Creation failed: %v", err)
		return nil, err
	default:
		err := fmt.Errorf("unsupported backend type '%s' for token bucket for key '%s'", cfg.Backend, cfg.Key)
		log.Printf("Creation failed: %v", err)
		return nil, err
	}
}
