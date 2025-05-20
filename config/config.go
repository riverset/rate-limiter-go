package config

import "time"

// AlgorithmType represents the type of rate limiting algorithm.
type AlgorithmType string

const (
	FixedWindowCounter   AlgorithmType = "fixed_window_counter"
	SlidingWindowCounter AlgorithmType = "sliding_window_counter"
	TokenBucket          AlgorithmType = "token_bucket"
)

// BackendType represents the storage backend.
type BackendType string

const (
	InMemory BackendType = "in_memory"
	Redis    BackendType = "redis"
	Memcache BackendType = "memcache"
)

// LimiterConfig holds the configuration for a single rate limiter instance.
type LimiterConfig struct {
	Algorithm AlgorithmType `yaml:"algorithm"`
	Backend   BackendType   `yaml:"backend"`
	Key       string        `yaml:"key"`

	WindowParams      *WindowConfig      `yaml:"window_params,omitempty"`
	TokenBucketParams *TokenBucketConfig `yaml:"token_bucket_params,omitempty"`

	RedisParams    *RedisBackendConfig    `yaml:"redis_params,omitempty"`
	MemcacheParams *MemcacheBackendConfig `yaml:"memcache_params,omitempty"`
}

// FixedWindowCounterConfig holds parameters for the Fixed Window Counter algorithm.
type WindowConfig struct {
	Window time.Duration `yaml:"window"`
	Limit  int64         `yaml:"limit"`
}

// TokenBucketConfig holds parameters for the Token Bucket algorithm.
type TokenBucketConfig struct {
	Rate     int `yaml:"rate"`
	Capacity int `yaml:"capacity"`
}

// RedisBackendConfig holds parameters for the Redis backend.
type RedisBackendConfig struct {
	Address  string `yaml:"address"`
	Password string `yaml:"password,omitempty"`
	DB       int    `yaml:"db,omitempty"`
}

// MemcacheBackendConfig holds parameters for the Memcache backend.
type MemcacheBackendConfig struct {
	Addresses []string `yaml:"addresses"`
}
