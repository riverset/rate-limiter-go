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

// loadConfig reads and unmarshals the YAML config.
// It now expects a list of limiters under the 'limiters' key.
func LoadConfig(path string) (*ConfigFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg ConfigFile // Unmarshal into the new struct
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	return &cfg, nil
}

// initRedisClient initializes and pings a Redis client based on config.
func InitRedisClient(cfg *config.LimiterConfig) (*redis.Client, error) {
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
	if _, err := client.Ping(ctx).Result(); err != nil {
		log.Printf("Redis ping failed: %v", err)
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}
	log.Println("Connected to Redis successfully during limiter initialization")
	return client, nil
}
