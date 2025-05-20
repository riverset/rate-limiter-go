package api

import (
	"fmt"
	"io"
	"log"

	"github.com/go-redis/redis/v8"
	apiinternal "learn.ratelimiter/api/internal"
	"learn.ratelimiter/config"
	"learn.ratelimiter/types"
)

// clientCloser is an internal type that holds backend clients and implements io.Closer.
type clientCloser struct {
	clients types.BackendClients
}

// Close gracefully shuts down all initialized backend clients held by the clientCloser.
func (c *clientCloser) Close() error {
	log.Println("API: Starting backend client shutdown...")
	var errs []error

	if c.clients.RedisClient != nil {
		log.Println("API: Closing Redis client...")
		if err := c.clients.RedisClient.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close Redis client: %w", err))
			log.Printf("API: Error closing Redis client: %v", err)
		} else {
			log.Println("API: Redis client closed successfully.")
		}
	}

	// Add closing logic for other clients (e.g., Memcache) here
	// if c.clients.MemcacheClient != nil {
	// 	log.Println("API: Closing Memcache client...")
	// 	if err := c.clients.MemcacheClient.Close(); err != nil {
	// 		errs = append(errs, fmt.Errorf("failed to close Memcache client: %w", err))
	// 		log.Printf("API: Error closing Memcache client: %v", err)
	// 	} else {
	// 		log.Println("API: Memcache client closed successfully.")
	// 	}
	// }

	if len(errs) > 0 {
		// Consider using a dedicated multi-error type for better handling
		return fmt.Errorf("errors during client shutdown: %v", errs)
	}

	log.Println("API: Backend client shutdown complete.")
	return nil
}

// NewLimitersFromConfigPath loads config, initializes any needed backend clients,
// and returns a map of rate limiters and an io.Closer for backend clients.
func NewLimitersFromConfigPath(configPath string) (map[string]types.Limiter, io.Closer, error) {
	log.Printf("API: Starting initialization of rate limiters from config path: %s", configPath)
	cfgFile, err := apiinternal.LoadConfig(configPath)
	if err != nil {
		// Improved error log
		log.Printf("API: Initialization failed: Error loading configuration from %s: %v", configPath, err)
		return nil, nil, fmt.Errorf("error loading configuration: %w", err)
	}

	if len(cfgFile.Limiters) == 0 {
		// Improved log
		log.Printf("API: Initialization failed: No limiter configurations found in %s", configPath)
		return nil, nil, fmt.Errorf("no limiter configurations found in %s", configPath)
	}

	backendClients := types.BackendClients{}
	var redisClient *redis.Client

	needsRedis := false
	for _, cfg := range cfgFile.Limiters {
		if cfg.Backend == config.Redis {
			needsRedis = true
			break
		}
	}

	if needsRedis {
		log.Println("API: Redis backend required for one or more limiters. Initializing Redis client...")
		var redisCfg *config.LimiterConfig
		for _, cfg := range cfgFile.Limiters {
			if cfg.Backend == config.Redis {
				redisCfg = &cfg
				break
			}
		}
		if redisCfg == nil {
			// This case should ideally not happen if needsRedis is true, but as a safeguard
			err := fmt.Errorf("logic error: needsRedis is true but no Redis config found")
			log.Printf("API: Initialization failed: %v", err)
			return nil, nil, err
		}

		redisClient, err = apiinternal.InitRedisClient(redisCfg)
		if err != nil {
			// Improved error log
			log.Printf("API: Initialization failed: Failed to initialize Redis client: %v", err)
			return nil, nil, err // initRedisClient already wraps the error
		}
		backendClients.RedisClient = redisClient
	}

	// Add initialization for other backends here if needed by the config
	// if anyCfg.Backend == config.Memcache { ... }

	limiters := make(map[string]types.Limiter)

	log.Printf("API: Creating %d limiter instances...", len(cfgFile.Limiters))
	for _, cfg := range cfgFile.Limiters {
		log.Printf("API: Creating limiter '%s' (Algorithm: %s, Backend: %s)...", cfg.Key, cfg.Algorithm, cfg.Backend)
		if cfg.Key == "" {
			err := fmt.Errorf("limiter configuration missing 'key' field")
			// Improved error log
			log.Printf("API: Initialization failed for a limiter: %v", err)
			return nil, nil, err
		}

		limiterFactory, err := NewLimiterFactory(cfg)
		if err != nil {
			err = fmt.Errorf("limiter '%s': failed to get factory: %w", cfg.Key, err)
			// Improved error log
			log.Printf("API: Initialization failed for limiter '%s': %v", cfg.Key, err)
			return nil, nil, err
		}

		limiter, err := limiterFactory.CreateLimiter(cfg, backendClients)
		if err != nil {
			err = fmt.Errorf("limiter '%s': failed to create instance: %w", cfg.Key, err)
			// Improved error log
			log.Printf("API: Initialization failed for limiter '%s': %v", cfg.Key, err)
			return nil, nil, err
		}

		limiters[cfg.Key] = limiter
		// Improved success log
		log.Printf("API: Limiter '%s' (Algorithm: %s, Backend: %s) created successfully.", cfg.Key, cfg.Algorithm, cfg.Backend)
	}

	log.Println("API: All rate limiters initialized.")

	closer := &clientCloser{clients: backendClients}
	return limiters, closer, nil
}

// You could also add a function that takes the config struct directly:
// func NewLimitersFromConfigStruct(cfg ConfigFile) (map[string]types.Limiter, io.Closer, error) { ... }
