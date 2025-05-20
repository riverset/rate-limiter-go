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
	return &cfg, nil
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
		Addr:     cfg.RedisParams.Address,
		Password: cfg.RedisParams.Password,
		DB:       cfg.RedisParams.DB,
		// Add other options like PoolSize, DialTimeout, ReadTimeout, WriteTimeout
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
