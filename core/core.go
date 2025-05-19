package core

import (
	"context" // Import context

	"github.com/go-redis/redis/v8"
)

// Limiter is the interface that all rate limiting algorithms must implement.
type Limiter interface {
	Allow(ctx context.Context, key string) (bool, error) // Added context parameter
}

// BackendClients holds initialized backend client instances.
type BackendClients struct {
	RedisClient *redis.Client
}
