package config

import "time"

// AlgorithmType represents the type of rate limiting algorithm.
type AlgorithmType string

const (
	FixedWindowCounter AlgorithmType = "fixed_window_counter"
	// Add other algorithm types here, e.g., TokenBucket AlgorithmType = "token_bucket"
)

// BackendType represents the storage backend.
type BackendType string

const (
	InMemory BackendType = "in_memory"
	Redis    BackendType = "redis"
	Memcache BackendType = "memcache" // Add Memcache backend type
	// Add other backend types here
)

// LimiterConfig holds the configuration for a single rate limiter instance.
type LimiterConfig struct {
	Algorithm AlgorithmType `yaml:"algorithm"`
	Backend   BackendType   `yaml:"backend"`
	Key       string        `yaml:"key"` // Identifier for this specific limiter instance (e.g., "api_rate_limit")

	// Parameters specific to algorithms
	FixedWindowCounterParams *FixedWindowCounterConfig `yaml:"fixed_window_counter_params,omitempty"`
	// Add parameters for other algorithms here, e.g., TokenBucketParams *TokenBucketConfig `yaml:"token_bucket_params,omitempty"`

	// Parameters specific to backends
	RedisParams    *RedisBackendConfig    `yaml:"redis_params,omitempty"`
	MemcacheParams *MemcacheBackendConfig `yaml:"memcache_params,omitempty"` // Add Memcache params
	// Add parameters for other backends here
}

// FixedWindowCounterConfig holds parameters for the Fixed Window Counter algorithm.
type FixedWindowCounterConfig struct {
	Window time.Duration `yaml:"window"`
	Limit  int64         `yaml:"limit"`
}

// RedisBackendConfig holds parameters for the Redis backend.
type RedisBackendConfig struct {
	Address  string `yaml:"address"`
	Password string `yaml:"password,omitempty"`
	DB       int    `yaml:"db,omitempty"`
	// Add other Redis client options here
}

// MemcacheBackendConfig holds parameters for the Memcache backend.
type MemcacheBackendConfig struct {
	Addresses []string `yaml:"addresses"`
	// Add other Memcache client options here
}

// GlobalConfig could hold multiple limiter configurations if needed
// type GlobalConfig struct {
// 	Limiters []LimiterConfig `yaml:"limiters"`
// }
