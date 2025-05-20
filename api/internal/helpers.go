package internal

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	"gopkg.in/yaml.v2"

	"learn.ratelimiter/config"
)

// ConfigFile represents the top-level structure of the configuration file.
type ConfigFile struct {
	Limiters []config.LimiterConfig `yaml:"limiters"`
}

// LoadConfig reads and unmarshals the YAML config.
// It now expects a list of limiters under the 'limiters' key.
func LoadConfig(path string) (*ConfigFile, error) {
	log.Printf("Helpers: Attempting to load configuration from %s", path)
	data, err := os.ReadFile(path)
	if err != nil {
		// Improved error log
		log.Printf("Helpers: Failed to read config file %s: %v", path, err)
		return nil, fmt.Errorf("read config file %s: %w", path, err)
	}
	var cfg ConfigFile // Unmarshal into the new struct
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		// Improved error log
		log.Printf("Helpers: Failed to unmarshal config file %s: %v", path, err)
		return nil, fmt.Errorf("unmarshal config file %s: %w", path, err)
	}
	log.Printf("Helpers: Configuration loaded successfully from %s", path)
	return &cfg, nil
}

// InitRedisClient initializes and pings a Redis client based on config.
func InitRedisClient(cfg *config.LimiterConfig) (*redis.Client, error) {
	log.Printf("Helpers: Attempting to initialize Redis client for address %s, DB %d", cfg.RedisParams.Address, cfg.RedisParams.DB)
	if cfg.RedisParams == nil {
		err := fmt.Errorf("redis backend selected but redis_params are missing in config")
		log.Printf("Helpers: Redis initialization failed: %v", err)
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
	log.Printf("Helpers: Pinging Redis at %s...", cfg.RedisParams.Address)
	if _, err := client.Ping(ctx).Result(); err != nil {
		// Improved error log
		log.Printf("Helpers: Failed to connect to Redis at %s: Ping failed: %v", cfg.RedisParams.Address, err)
		// Close the client if ping fails to prevent resource leaks
		client.Close()
		return nil, fmt.Errorf("failed to connect to Redis at %s: %w", cfg.RedisParams.Address, err)
	}
	log.Printf("Helpers: Successfully connected to Redis at %s.", cfg.RedisParams.Address)
	return client, nil
}
