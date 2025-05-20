// Package internal contains internal helper functions for the API.
package internal

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/rs/zerolog/log" // Import zerolog's global logger
	"gopkg.in/yaml.v2"

	"learn.ratelimiter/config"
)

// ConfigFile represents the top-level structure of the configuration file.
// It contains a list of rate limiter configurations.
type ConfigFile struct {
	// Limiters is a list of individual rate limiter configurations.
	Limiters []config.LimiterConfig `yaml:"limiters"`
}

// LoadConfig reads and unmarshals the YAML configuration file from the given path.
// It returns a ConfigFile struct or an error if loading or unmarshalling fails.
func LoadConfig(path string) (*ConfigFile, error) {
	log.Info().Str("config_path", path).Msg("Helpers: Attempting to load configuration")
	data, err := os.ReadFile(path)
	if err != nil {
		// Improved error log with structured fields
		log.Error().Err(err).Str("config_path", path).Msg("Helpers: Failed to read config file")
		return nil, fmt.Errorf("read config file %s: %w", path, err)
	}
	var cfg ConfigFile // Unmarshal into the new struct
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		// Improved error log with structured fields
		log.Error().Err(err).Str("config_path", path).Msg("Helpers: Failed to unmarshal config file")
		return nil, fmt.Errorf("unmarshal config file %s: %w", path, err)
	}
	log.Info().Str("config_path", path).Msg("Helpers: Configuration loaded successfully")

	// Validate the loaded configuration
	log.Info().Msg("Helpers: Validating configuration")
	if err := validateConfig(&cfg); err != nil {
		log.Error().Err(err).Msg("Helpers: Configuration validation failed")
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	log.Info().Msg("Helpers: Configuration validated successfully")
	return &cfg, nil
}

// validateConfig performs validation checks on the loaded configuration.
func validateConfig(cfg *ConfigFile) error {
	if cfg == nil || len(cfg.Limiters) == 0 {
		return fmt.Errorf("no rate limiters defined in configuration")
	}

	for _, limiterCfg := range cfg.Limiters {
		if limiterCfg.Key == "" {
			return fmt.Errorf("limiter key is required for all limiters")
		}

		switch limiterCfg.Algorithm {
		case config.TokenBucket:
			if limiterCfg.TokenBucketParams == nil {
				return fmt.Errorf("token_bucket_params are required for token_bucket limiter '%s'", limiterCfg.Key)
			}
			if limiterCfg.TokenBucketParams.Rate <= 0 {
				return fmt.Errorf("rate must be a positive integer for token_bucket limiter '%s'", limiterCfg.Key)
			}
			if limiterCfg.TokenBucketParams.Capacity <= 0 {
				return fmt.Errorf("capacity must be a positive integer for token_bucket limiter '%s'", limiterCfg.Key)
			}
		case config.FixedWindowCounter, config.SlidingWindowCounter:
			if limiterCfg.WindowParams == nil {
				return fmt.Errorf("window_params are required for %s limiter '%s'", limiterCfg.Algorithm, limiterCfg.Key)
			}
			if limiterCfg.WindowParams.Window <= 0 {
				return fmt.Errorf("window duration must be positive for %s limiter '%s'", limiterCfg.Algorithm, limiterCfg.Key)
			}
			if limiterCfg.WindowParams.Limit <= 0 {
				return fmt.Errorf("limit must be a positive integer for %s limiter '%s'", limiterCfg.Algorithm, limiterCfg.Key)
			}
		default:
			return fmt.Errorf("unsupported algorithm type '%s' for limiter '%s'", limiterCfg.Algorithm, limiterCfg.Key)
		}

		switch limiterCfg.Backend {
		case config.InMemory:
			// No specific backend params to validate for in-memory
		case config.Redis:
			if limiterCfg.RedisParams == nil {
				return fmt.Errorf("redis_params are required for redis backend for limiter '%s'", limiterCfg.Key)
			}
			if limiterCfg.RedisParams.Address == "" {
				return fmt.Errorf("redis address is required for redis backend for limiter '%s'", limiterCfg.Key)
			}
		case config.Memcache:
			if limiterCfg.MemcacheParams == nil {
				return fmt.Errorf("memcache_params are required for memcache backend for limiter '%s'", limiterCfg.Key)
			}
			if len(limiterCfg.MemcacheParams.Addresses) == 0 || limiterCfg.MemcacheParams.Addresses[0] == "" {
				return fmt.Errorf("at least one memcache address is required for memcache backend for limiter '%s'", limiterCfg.Key)
			}
		default:
			return fmt.Errorf("unsupported backend type '%s' for limiter '%s'", limiterCfg.Backend, limiterCfg.Key)
		}
	}

	return nil
}

// InitRedisClient initializes and pings a Redis client based on the provided limiter configuration.
// It takes a LimiterConfig (specifically the RedisParams) and returns a Redis client instance or an error.
func InitRedisClient(cfg *config.LimiterConfig) (*redis.Client, error) {
	log.Info().Str("address", cfg.RedisParams.Address).Int("db", cfg.RedisParams.DB).Msg("Helpers: Attempting to initialize Redis client")
	if cfg.RedisParams == nil {
		err := fmt.Errorf("redis backend selected but redis_params are missing in config")
		log.Error().Err(err).Msg("Helpers: Redis initialization failed")
		return nil, err
	}
	client := redis.NewClient(&redis.Options{
		Addr:         cfg.RedisParams.Address,
		Password:     cfg.RedisParams.Password,
		DB:           cfg.RedisParams.DB,
		PoolSize:     cfg.RedisParams.PoolSize,
		DialTimeout:  cfg.RedisParams.DialTimeout,
		ReadTimeout:  cfg.RedisParams.ReadTimeout,
		WriteTimeout: cfg.RedisParams.WriteTimeout,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	log.Info().Str("address", cfg.RedisParams.Address).Msg("Helpers: Pinging Redis...")
	if _, err := client.Ping(ctx).Result(); err != nil {
		// Improved error log with structured fields
		log.Error().Err(err).Str("address", cfg.RedisParams.Address).Msg("Helpers: Failed to connect to Redis: Ping failed")
		// Close the client if ping fails to prevent resource leaks
		client.Close()
		return nil, fmt.Errorf("failed to connect to Redis at %s: %w", cfg.RedisParams.Address, err)
	}
	log.Info().Str("address", cfg.RedisParams.Address).Msg("Helpers: Successfully connected to Redis.")
	return client, nil
}
