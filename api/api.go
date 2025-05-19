package api

import (
	"context"
	"fmt"
	"log" // Consider using a configurable logger instead of the standard log package
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	"gopkg.in/yaml.v2"

	"learn.ratelimiter/config"
	"learn.ratelimiter/core" // added to reference shared types
)

// loadConfig reads the configuration from a YAML file.
// Moved from main.go
func loadConfig(filepath string) (*config.LimiterConfig, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg struct {
		Limiter config.LimiterConfig `yaml:"limiter"`
	}
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg.Limiter, nil
}

// NewLimiterFromConfigPath creates and initializes a Limiter instance
// based on the configuration found at the given file path.
// This is the main entry point for consumers of the rate limiter package.
func NewLimiterFromConfigPath(configPath string) (Limiter, error) {
	// Load configuration
	cfg, err := loadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("error loading configuration: %w", err)
	}

	// Initialize backend clients based on config using core.BackendClients
	backendClients := core.BackendClients{} // updated here

	if cfg.Backend == config.Redis {
		if cfg.RedisParams == nil {
			return nil, fmt.Errorf("redis backend selected but redis_params are missing in config")
		}
		redisClient := redis.NewClient(&redis.Options{
			Addr:     cfg.RedisParams.Address,
			Password: cfg.RedisParams.Password,
			DB:       cfg.RedisParams.DB,
			// Add other options like PoolSize, DialTimeout, ReadTimeout, WriteTimeout
		})

		// Ping Redis to check connection
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err := redisClient.Ping(ctx).Result()
		if err != nil {
			// Log the error but return it so the caller knows initialization failed
			log.Printf("Failed to connect to Redis: %v", err)
			return nil, fmt.Errorf("failed to connect to Redis: %w", err)
		}
		log.Println("Connected to Redis successfully during limiter initialization")
		backendClients.RedisClient = redisClient // Add the initialized client to the struct
	}

	// Use the new factory that returns a strategy for the algorithm.
	factory, err := NewFactory(*cfg)
	if err != nil {
		return nil, err
	}

	// Create limiter instance using the factory, config, and initialized clients
	rateLimiter, err := factory.CreateLimiter(*cfg, backendClients) // Pass the clients struct
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

	return rateLimiter, nil
}

// You could also add a function that takes the config struct directly:
// func NewLimiterFromConfigStruct(cfg config.LimiterConfig) (Limiter, error) { ... }
