package core

import "github.com/go-redis/redis/v8"

// Limiter is the interface that all rate limiting algorithms must implement.
type Limiter interface {
	Allow(key string) (bool, error)
}

// BackendClients holds initialized backend client instances.
type BackendClients struct {
	RedisClient *redis.Client
}
