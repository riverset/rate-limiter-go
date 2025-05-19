package api

import (
	"fmt"
	"io" // Import io for the Closer interface
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
	var errs []error

	if c.clients.RedisClient != nil {
		log.Println("Closing Redis client...")
		if err := c.clients.RedisClient.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close Redis client: %w", err))
		} else {
			log.Println("Redis client closed.")
		}
	}

	// Add closing logic for other clients (e.g., Memcache) here
	// if c.clients.MemcacheClient != nil {
	// 	log.Println("Closing Memcache client...")
	// 	if err := c.clients.MemcacheClient.Close(); err != nil {
	// 		errs = append(errs, fmt.Errorf("failed to close Memcache client: %w", err))
	// 	} else {
	// 		log.Println("Memcache client closed.")
	// 	}
	// }

	if len(errs) > 0 {
		// You might want to return a multi-error type here in a real library
		return fmt.Errorf("errors during client shutdown: %v", errs)
	}

	return nil
}

// NewLimitersFromConfigPath loads config, initializes any needed backend clients,
// and returns a map of rate limiters and an io.Closer for backend clients.
func NewLimitersFromConfigPath(configPath string) (map[string]types.Limiter, io.Closer, error) { // Updated return type
	cfgFile, err := apiinternal.LoadConfig(configPath)
	if err != nil {
		return nil, nil, fmt.Errorf("error loading configuration: %w", err) // Updated return
	}

	if len(cfgFile.Limiters) == 0 {
		return nil, nil, fmt.Errorf("no limiter configurations found in %s", configPath) // Updated return
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
		var redisCfg *config.LimiterConfig
		for _, cfg := range cfgFile.Limiters {
			if cfg.Backend == config.Redis {
				redisCfg = &cfg
				break
			}
		}
		if redisCfg == nil {
			return nil, nil, fmt.Errorf("logic error: needsRedis is true but no Redis config found") // Updated return
		}

		redisClient, err = apiinternal.InitRedisClient(redisCfg)
		if err != nil {
			return nil, nil, err // Updated return
		}
		backendClients.RedisClient = redisClient
	}

	// Add initialization for other backends here if needed by the config
	// if anyCfg.Backend == config.Memcache { ... }

	limiters := make(map[string]types.Limiter)

	for _, cfg := range cfgFile.Limiters {
		if cfg.Key == "" {
			return nil, nil, fmt.Errorf("limiter configuration missing 'key' field") // Updated return
		}

		factory, err := NewFactory(cfg)
		if err != nil {
			return nil, nil, fmt.Errorf("limiter '%s': %w", cfg.Key, err) // Updated return
		}

		limiter, err := factory.CreateLimiter(cfg, backendClients)
		if err != nil {
			return nil, nil, fmt.Errorf("limiter '%s': %w", cfg.Key, err) // Updated return
		}

		limiters[cfg.Key] = limiter
		log.Printf("Limiter '%s' initialized: Algorithm=%s, Backend=%s", cfg.Key, cfg.Algorithm, cfg.Backend)
	}

	// Return the map of limiters and the clientCloser
	closer := &clientCloser{clients: backendClients}
	return limiters, closer, nil // Updated return
}

// You could also add a function that takes the config struct directly:
// func NewLimitersFromConfigStruct(cfg ConfigFile) (map[string]types.Limiter, io.Closer, error) { ... }
