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
	log.Printf("Loading configuration from %s", path)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file %s: %w", path, err)
	}
	var cfg ConfigFile // Unmarshal into the new struct
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config file %s: %w", path, err)
	}
	log.Printf("Configuration loaded successfully from %s", path)
	return &cfg, nil
}

// InitRedisClient initializes and pings a Redis client based on config.
func InitRedisClient(cfg *config.LimiterConfig) (*redis.Client, error) {
	log.Printf("Initializing Redis client for address %s, DB %d", cfg.RedisParams.Address, cfg.RedisParams.DB)
	if cfg.RedisParams == nil {
		return nil, fmt.Errorf("redis backend selected but redis_params are missing in config")
	}
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisParams.Address,
		Password: cfg.RedisParams.Password,
		DB:       cfg.RedisParams.DB,
		// Add other options like PoolSize, DialTimeout, ReadTimeout, WriteTimeout
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	log.Println("Pinging Redis...")
	if _, err := client.Ping(ctx).Result(); err != nil {
		log.Printf("Redis ping failed for %s: %v", cfg.RedisParams.Address, err)
		// Close the client if ping fails to prevent resource leaks
		client.Close()
		return nil, fmt.Errorf("failed to connect to Redis at %s: %w", cfg.RedisParams.Address, err)
	}
	log.Println("Connected to Redis successfully.")
	return client, nil
}
