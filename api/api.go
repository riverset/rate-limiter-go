package api

import (
	"fmt"
	"log"

	"github.com/go-redis/redis/v8" // Import redis for client type
	apiinternal "learn.ratelimiter/api/internal"
	"learn.ratelimiter/config"
	"learn.ratelimiter/types"
)

// NewLimitersFromConfigPath loads config, initializes any needed backend clients,
// and returns a map of rate limiters keyed by their configuration key.
func NewLimitersFromConfigPath(configPath string) (map[string]types.Limiter, error) {
	// Use the helper from the internal package to load all configs
	cfgFile, err := apiinternal.LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("error loading configuration: %w", err)
	}

	if len(cfgFile.Limiters) == 0 {
		return nil, fmt.Errorf("no limiter configurations found in %s", configPath)
	}

	// Initialize backend clients needed by *any* limiter
	// This avoids initializing clients multiple times if multiple limiters use the same backend.
	backendClients := types.BackendClients{}
	var redisClient *redis.Client // Use the concrete type here

	// Check if Redis is needed by any limiter
	needsRedis := false
	for _, cfg := range cfgFile.Limiters {
		if cfg.Backend == config.Redis {
			needsRedis = true
			break
		}
	}

	if needsRedis {
		// Find the first Redis config to initialize the client (assuming all Redis configs use the same client)
		// A more robust solution might handle different Redis configs/clients.
		var redisCfg *config.LimiterConfig
		for _, cfg := range cfgFile.Limiters {
			if cfg.Backend == config.Redis {
				redisCfg = &cfg
				break
			}
		}
		if redisCfg == nil {
			// This case should ideally not happen if needsRedis is true, but as a safeguard
			return nil, fmt.Errorf("logic error: needsRedis is true but no Redis config found")
		}

		redisClient, err = apiinternal.InitRedisClient(redisCfg)
		if err != nil {
			return nil, err // initRedisClient already wraps the error
		}
		backendClients.RedisClient = redisClient
	}

	// Add initialization for other backends here if needed by the config
	// if anyCfg.Backend == config.Memcache { ... }

	// Create a map to hold the initialized limiters
	limiters := make(map[string]types.Limiter)

	// Iterate through each limiter configuration and create the limiter instance
	for _, cfg := range cfgFile.Limiters {
		if cfg.Key == "" {
			return nil, fmt.Errorf("limiter configuration missing 'key' field")
		}

		// Get the appropriate factory for this limiter's algorithm
		factory, err := NewFactory(cfg)
		if err != nil {
			// Include the key in the error for easier debugging
			return nil, fmt.Errorf("limiter '%s': %w", cfg.Key, err)
		}

		// Create the limiter instance using the factory and the initialized clients
		limiter, err := factory.CreateLimiter(cfg, backendClients)
		if err != nil {
			// Include the key in the error for easier debugging
			return nil, fmt.Errorf("limiter '%s': %w", cfg.Key, err)
		}

		// Store the created limiter in the map
		limiters[cfg.Key] = limiter
		log.Printf("Limiter '%s' initialized: Algorithm=%s, Backend=%s", cfg.Key, cfg.Algorithm, cfg.Backend)
	}

	// Note: Backend clients (like redisClient) are created here.
	// If the consumer needs to gracefully shut down, this function might need
	// to return the clients as well, or provide a Close() method on the Limiter
	// interface that handles closing underlying connections.
	// For simplicity now, we won't return clients, assuming they might be managed
	// internally by the limiter implementation or left open. A robust library
	// would need a shutdown mechanism.

	return limiters, nil
}

// You could also add a function that takes the config struct directly:
// func NewLimitersFromConfigStruct(cfg ConfigFile) (map[string]Limiter, error) { ... }
