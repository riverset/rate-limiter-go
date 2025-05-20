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
		log.Printf("Factory(TokenBucket): Creation failed for key '%s': %v", cfg.Key, err)
		return nil, err
	}

	switch cfg.Backend {
	case config.InMemory:
		// Added parameters to log
		log.Printf("Factory(TokenBucket): Creating in-memory limiter for key '%s' with rate %d, capacity %d", cfg.Key, cfg.TokenBucketParams.Rate, cfg.TokenBucketParams.Capacity)
		// Assuming the inmemory package has a New function matching the signature
		return tbinmemory.NewLimiter(cfg.Key, cfg.TokenBucketParams.Rate, cfg.TokenBucketParams.Capacity), nil // Pass key to in-memory limiter
	case config.Redis:
		// Added parameters to log
		log.Printf("Factory(TokenBucket): Creating Redis limiter for key '%s' with rate %d, capacity %d", cfg.Key, cfg.TokenBucketParams.Rate, cfg.TokenBucketParams.Capacity)
		if clients.RedisClient == nil {
			err := fmt.Errorf("redis client is required but not provided for redis backend for key '%s'", cfg.Key)
			log.Printf("Factory(TokenBucket): Creation failed for key '%s': %v", cfg.Key, err)
			return nil, err
		}
		return redistb.NewLimiter(cfg.Key, cfg.TokenBucketParams.Rate, cfg.TokenBucketParams.Capacity, clients.RedisClient), nil

	case config.Memcache:
		err := fmt.Errorf("memcache backend not yet implemented for token bucket for key '%s'", cfg.Key)
		log.Printf("Factory(TokenBucket): Creation failed for key '%s': %v", cfg.Key, err)
		return nil, err
	default:
		err := fmt.Errorf("unsupported backend type '%s' for token bucket for key '%s'", cfg.Backend, cfg.Key)
		log.Printf("Factory(TokenBucket): Creation failed for key '%s': %v", cfg.Key, err)
		return nil, err
	}
}
