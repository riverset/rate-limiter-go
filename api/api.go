// Package api provides the main interface for initializing and using the rate limiters.
package api

import (
	"fmt"
	"io"

	// Import time for zerolog
	"github.com/go-redis/redis/v8"
	"github.com/rs/zerolog/log" // Import zerolog's global logger

	apiinternal "learn.ratelimiter/api/internal"
	"learn.ratelimiter/config"
	"learn.ratelimiter/types"
)

// clientCloser is an internal type that holds backend clients and implements io.Closer.
type clientCloser struct {
	clients types.BackendClients
}

// Close gracefully shuts down all initialized backend clients held by the clientCloser.
// It returns an error if any client fails to close.
func (c *clientCloser) Close() error {
	log.Info().Msg("API: Starting backend client shutdown...")
	var errs []error

	if c.clients.RedisClient != nil {
		log.Info().Msg("API: Closing Redis client...")
		if err := c.clients.RedisClient.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close Redis client: %w", err))
			log.Error().Err(err).Msg("API: Error closing Redis client")
		} else {
			log.Info().Msg("API: Redis client closed successfully.")
		}
	}

	// Add closing logic for other clients (e.g., Memcache) here
	// if c.clients.MemcacheClient != nil {
	// 	log.Info().Msg("API: Closing Memcache client...")
	// 	if err := c.clients.MemcacheClient.Close(); err != nil {
	// 		errs = append(errs, fmt.Errorf("failed to close Memcache client: %w", err))
	// 		log.Error().Err(err).Msg("API: Error closing Memcache client")
	// 	} else {
	// 		log.Info().Msg("API: Memcache client closed successfully.")
	// 	}
	// }

	if len(errs) > 0 {
		// Consider using a dedicated multi-error type for better handling
		return fmt.Errorf("errors during client shutdown: %v", errs)
	}

	log.Info().Msg("API: Backend client shutdown complete.")
	return nil
}

// NewLimitersFromConfigPath loads configuration from the given path, initializes any needed backend clients,
// and returns a map of rate limiters keyed by their configuration key, a map of configurations keyed by their key, and an io.Closer for backend clients.
// It returns an error if configuration loading or client/limiter initialization fails.
func NewLimitersFromConfigPath(configPath string) (map[string]types.Limiter, map[string]config.LimiterConfig, io.Closer, error) {
	log.Info().Str("config_path", configPath).Msg("API: Starting initialization of rate limiters from config path")
	cfgFile, err := apiinternal.LoadConfig(configPath)
	if err != nil {
		// Improved error log with structured fields
		log.Error().Err(err).Str("config_path", configPath).Msg("API: Initialization failed: Error loading configuration")
		return nil, nil, nil, fmt.Errorf("error loading configuration: %w", err)
	}

	if len(cfgFile.Limiters) == 0 {
		// Improved log with structured fields
		log.Error().Str("config_path", configPath).Msg("API: Initialization failed: No limiter configurations found")
		return nil, nil, nil, fmt.Errorf("no limiter configurations found in %s", configPath)
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
		log.Info().Msg("API: Redis backend required for one or more limiters. Initializing Redis client...")
		// Find the first Redis config to initialize the client
		var redisCfg *config.LimiterConfig
		for _, cfg := range cfgFile.Limiters {
			if cfg.Backend == config.Redis && cfg.RedisParams != nil {
				redisCfg = &cfg
				break
			}
		}
		// If no Redis config with params is found, return an error
		if redisCfg == nil {
			err := fmt.Errorf("redis backend specified but no valid redis_params found in config")
			log.Error().Err(err).Msg("API: Initialization failed")
			return nil, nil, nil, err
		}

		redisClient, err = apiinternal.InitRedisClient(redisCfg)
		if err != nil {
			// Improved error log with structured fields
			log.Error().Err(err).Msg("API: Initialization failed: Failed to initialize Redis client")
			return nil, nil, nil, err // initRedisClient already wraps the error
		}
		backendClients.RedisClient = redisClient
	}

	// Add initialization for other backends here if needed by the config
	// if anyCfg.Backend == config.Memcache { ... }

	limiters := make(map[string]types.Limiter)
	limiterConfigs := make(map[string]config.LimiterConfig)

	log.Info().Int("count", len(cfgFile.Limiters)).Msg("API: Creating limiter instances...")
	for _, cfg := range cfgFile.Limiters {
		log.Info().Str("limiter_key", cfg.Key).Str("algorithm", string(cfg.Algorithm)).Str("backend", string(cfg.Backend)).Msg("API: Creating limiter...")
		if cfg.Key == "" {
			err := fmt.Errorf("limiter configuration missing 'key' field")
			// Improved error log with structured fields
			log.Error().Err(err).Msg("API: Initialization failed for a limiter")
			return nil, nil, nil, err
		}

		limiterFactory, err := NewLimiterFactory(cfg)
		if err != nil {
			err = fmt.Errorf("limiter '%s': failed to get factory: %w", cfg.Key, err)
			// Improved error log with structured fields
			log.Error().Err(err).Str("limiter_key", cfg.Key).Msg("API: Initialization failed: Failed to get factory")
			return nil, nil, nil, err
		}

		limiter, err := limiterFactory.CreateLimiter(cfg, backendClients)
		if err != nil {
			err = fmt.Errorf("limiter '%s': failed to create instance: %w", cfg.Key, err)
			// Improved error log with structured fields
			log.Error().Err(err).Str("limiter_key", cfg.Key).Msg("API: Initialization failed: Failed to create instance")
			return nil, nil, nil, err
		}

		limiters[cfg.Key] = limiter
		limiterConfigs[cfg.Key] = cfg // Store the config as well
		// Improved success log with structured fields
		log.Info().Str("limiter_key", cfg.Key).Str("algorithm", string(cfg.Algorithm)).Str("backend", string(cfg.Backend)).Msg("API: Limiter created successfully.")
	}

	log.Info().Msg("API: All rate limiters initialized.")

	closer := &clientCloser{clients: backendClients}
	return limiters, limiterConfigs, closer, nil
}

// You could also add a function that takes the config struct directly:
// func NewLimitersFromConfigStruct(cfg ConfigFile) (map[string]types.Limiter, io.Closer, error) { ... }
