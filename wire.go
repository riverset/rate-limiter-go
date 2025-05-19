//go:build wireinject
// +build wireinject

// filepath: /Users/prakhar/Desktop/codingChallenges/rate-limiter-go/wire.go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/wire"
	"gopkg.in/yaml.v2" // Needed if loadConfig is used here

	"learn.ratelimiter/api"
	"learn.ratelimiter/config"
	"learn.ratelimiter/metrics"
	"learn.ratelimiter/middleware"
)

// Provider functions for components

// loadConfig reads the configuration from a YAML file.
// This function needs to be accessible to the wire providers.
func loadConfig(filepath string) (*config.LimiterConfig, error) {
	data, err := os.ReadFile(filepath) // os needs to be imported if used here
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

func provideLimiterConfig() (*config.LimiterConfig, error) {
	return loadConfig("config.yaml")
}

func provideRedisClient(cfg *config.LimiterConfig) (*redis.Client, error) {
	if cfg.Backend != config.Redis {
		return nil, nil // Return nil client if not needed
	}
	if cfg.RedisParams == nil {
		return nil, fmt.Errorf("redis backend selected but redis_params are missing")
	}
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisParams.Address,
		Password: cfg.RedisParams.Password,
		DB:       cfg.RedisParams.DB,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := client.Ping(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}
	log.Println("Connected to Redis successfully via Wire provider")
	return client, nil
}

func provideBackendClients(redisClient *redis.Client) api.BackendClients {
	return api.BackendClients{
		RedisClient: redisClient,
	}
}

func provideLimiterFactory() *api.Factory {
	return api.NewFactory()
}

func provideLimiter(factory *api.Factory, cfg *config.LimiterConfig, clients api.BackendClients) (api.Limiter, error) {
	return factory.CreateLimiter(*cfg, clients)
}

func provideMetrics() *metrics.RateLimitMetrics {
	return metrics.NewRateLimitMetrics()
}

func provideRateLimitMiddleware(limiter api.Limiter, metrics *metrics.RateLimitMetrics) *middleware.RateLimitMiddleware {
	// Note: getClientIP is not managed by Wire here.
	return middleware.NewRateLimitMiddleware(limiter, metrics)
}

// The Injector function declaration.
// This tells Wire what top-level components we want to build.
func InitializeApplication() (*application, error) {
	wire.Build(
		provideLimiterConfig,
		provideRedisClient,
		provideBackendClients,
		provideLimiterFactory,
		provideLimiter,
		provideMetrics,
		provideRateLimitMiddleware,
		// Provide the application struct itself
		wire.Struct(new(application), "*"),
	)
	return nil, nil // This return is only for Wire's analysis
}
