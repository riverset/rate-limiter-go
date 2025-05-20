// Package types defines common types and interfaces used throughout the rate limiter.
package types

import (
	"context" // Import context

	"github.com/go-redis/redis/v8"
)

// Limiter is the interface that all rate limiting algorithms must implement.
type Limiter interface {
	// Allow checks if a request is allowed for the given key.
	// It returns true if the request is allowed, false otherwise, and an error if any occurred.
	Allow(ctx context.Context, key string) (bool, error)
}

// BackendClients holds initialized backend client instances.
type BackendClients struct {
	// RedisClient is the Redis client instance.
	RedisClient *redis.Client
}
