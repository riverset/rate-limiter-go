package api

import (
	"fmt"
	"log"

	apiinternal "learn.ratelimiter/api/internal" // Import the new internal package
	"learn.ratelimiter/config"
	"learn.ratelimiter/core"
)

// NewLimiterFromConfigPath loads config, initializes any needed backend clients
// and returns a rate limiter.
func NewLimiterFromConfigPath(configPath string) (Limiter, error) {
	// Use the helper from the internal package
	cfg, err := apiinternal.LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("error loading configuration: %w", err)
	}

	clients := core.BackendClients{}
	if cfg.Backend == config.Redis {
		// Use the helper from the internal package
		redisClient, err := apiinternal.InitRedisClient(cfg)
		if err != nil {
			return nil, err
		}
		clients.RedisClient = redisClient
	}

	// Add initialization for other backends here if needed by the config
	// if cfg.Backend == config.Memcache { ... }

	factory, err := NewFactory(*cfg)
	if err != nil {
		return nil, err
	}

	limiter, err := factory.CreateLimiter(*cfg, clients)
	if err != nil {
		return nil, fmt.Errorf("error creating rate limiter instance: %w", err)
	}

	log.Printf("Rate limiter initialized: Algorithm=%s, Backend=%s, Key=%s", cfg.Algorithm, cfg.Backend, cfg.Key)
	if cfg.FixedWindowCounterParams != nil {
		log.Printf("  Fixed Window Params: Window=%s, Limit=%d", cfg.FixedWindowCounterParams.Window, cfg.FixedWindowCounterParams.Limit)
	}
	if cfg.RedisParams != nil {
		log.Printf("  Redis Params: Address=%s, DB=%d", cfg.RedisParams.Address, cfg.RedisParams.DB)
	}
	// Log other backend params if added

	// Note: Backend clients (like redisClient) are created here.
	// If the consumer needs to gracefully shut down, this function might need
	// to return the clients as well, or provide a Close() method on the Limiter
	// interface that handles closing underlying connections.
	// For simplicity now, we won't return clients, assuming they might be managed
	// internally by the limiter implementation or left open. A robust library
	// would need a shutdown mechanism.

	return limiter, nil
}

// You could also add a function that takes the config struct directly:
// func NewLimiterFromConfigStruct(cfg config.LimiterConfig) (Limiter, error) { ... }
