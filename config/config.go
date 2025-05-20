// Package config provides structures and logic for loading application configuration.
package config

import "time"

// AlgorithmType represents the type of rate limiting algorithm.
type AlgorithmType string

// Constants for supported rate limiting algorithms.
const (
	FixedWindowCounter   AlgorithmType = "fixed_window_counter"
	SlidingWindowCounter AlgorithmType = "sliding_window_counter"
	TokenBucket          AlgorithmType = "token_bucket"
	LeakyBucket          AlgorithmType = "leaky_bucket"
)

// BackendType represents the storage backend.
type BackendType string

// Constants for supported storage backends.
const (
	InMemory BackendType = "in_memory"
	Redis    BackendType = "redis"
	Memcache BackendType = "memcache"
)

// LimiterConfig holds the configuration for a single rate limiter instance.
type LimiterConfig struct {
	// Algorithm is the rate limiting algorithm to use (e.g., "token_bucket").
	Algorithm AlgorithmType `yaml:"algorithm"`
	// Backend is the storage backend to use (e.g., "in_memory", "redis").
	Backend BackendType `yaml:"backend"`
	// Key is a unique identifier for this rate limiter configuration.
	Key string `yaml:"key"`

	// WindowParams holds parameters for Fixed Window and Sliding Window algorithms.
	WindowParams *WindowConfig `yaml:"window_params,omitempty"`
	// TokenBucketParams holds parameters for the Token Bucket algorithm.
	TokenBucketParams *TokenBucketConfig `yaml:"token_bucket_params,omitempty"`
	// LeakyBucketParams holds parameters for the Leaky Bucket algorithm.
	LeakyBucketParams *LeakyBucketConfig `yaml:"leaky_bucket_params,omitempty"`

	// RedisParams holds configuration for the Redis backend.
	RedisParams *RedisBackendConfig `yaml:"redis_params,omitempty"`
	// MemcacheParams holds configuration for the Memcache backend.
	MemcacheParams *MemcacheBackendConfig `yaml:"memcache_params,omitempty"`
}

// WindowConfig holds parameters for the Fixed Window Counter and Sliding Window Counter algorithms.
type WindowConfig struct {
	// Window is the duration of the window in seconds.
	Window time.Duration `yaml:"window"`
	// Limit is the maximum number of requests allowed within the window.
	Limit int64 `yaml:"limit"`
}

// TokenBucketConfig holds parameters for the Token Bucket algorithm.
type TokenBucketConfig struct {
	// Rate is the number of tokens to add to the bucket per second.
	Rate int `yaml:"rate"`
	// Capacity is the maximum number of tokens the bucket can hold.
	Capacity int `yaml:"capacity"`
}

// LeakyBucketConfig holds parameters for the Leaky Bucket algorithm.
type LeakyBucketConfig struct {
	// Rate is the rate at which tokens leak from the bucket (tokens per second).
	Rate int `yaml:"rate"`
	// Capacity is the maximum number of tokens the bucket can hold.
	Capacity int `yaml:"capacity"`
}

// RedisBackendConfig holds parameters for the Redis backend.
type RedisBackendConfig struct {
	// Address is the address of the Redis server (e.g., "localhost:6379").
	Address string `yaml:"address"`
	// Password is the password for Redis authentication (optional).
	Password string `yaml:"password,omitempty"`
	// DB is the Redis database to use (optional).
	DB int `yaml:"db,omitempty"`
	// PoolSize is the maximum number of connections in the connection pool.
	PoolSize int `yaml:"pool_size,omitempty"`
	// DialTimeout is the timeout for establishing a connection.
	DialTimeout time.Duration `yaml:"dial_timeout,omitempty"`
	// ReadTimeout is the timeout for reading from the server.
	ReadTimeout time.Duration `yaml:"read_timeout,omitempty"`
	// WriteTimeout is the timeout for writing to the server.
	WriteTimeout time.Duration `yaml:"write_timeout,omitempty"`
}

// MemcacheBackendConfig holds parameters for the Memcache backend.
type MemcacheBackendConfig struct {
	// Addresses are the addresses of the Memcache servers.
	Addresses []string `yaml:"addresses"`
}
