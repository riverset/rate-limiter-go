package api

import (
	"fmt"

	"github.com/go-redis/redis/v8"
	"learn.ratelimiter/config"
	inmemoryfc "learn.ratelimiter/internal/fixedcounter/inmemory"
	redisfc "learn.ratelimiter/internal/fixedcounter/redis"
	// Add imports for other backends/algorithms
)

// BackendClients holds initialized backend client instances.
// Add fields for other backend clients as needed.
type BackendClients struct {
	RedisClient *redis.Client
	// MemcacheClient *memcache.Client // Add Memcache client field
	// DBClient *sql.DB // Add database client field
}

// Factory is responsible for creating Limiter instances based on configuration.
// It no longer holds backend clients directly.
type Factory struct {
	// Factory state if needed, but not backend clients
}

// NewFactory creates a new Factory instance.
// It no longer takes backend clients as dependencies.
func NewFactory() *Factory {
	return &Factory{}
}

// CreateLimiter creates a Limiter instance based on the provided configuration
// and available backend clients.
func (f *Factory) CreateLimiter(cfg config.LimiterConfig, clients BackendClients) (Limiter, error) {
	switch cfg.Algorithm {
	case config.FixedWindowCounter:
		if cfg.FixedWindowCounterParams == nil {
			return nil, fmt.Errorf("fixed window counter parameters are missing in config for key '%s'", cfg.Key)
		}
		switch cfg.Backend {
		case config.InMemory:
			// In-memory doesn't need external clients
			return inmemoryfc.NewLimiter(cfg.Key, cfg.FixedWindowCounterParams.Window, cfg.FixedWindowCounterParams.Limit), nil
		case config.Redis:
			if clients.RedisClient == nil {
				return nil, fmt.Errorf("redis client is required but not provided for redis backend for key '%s'", cfg.Key)
			}
			return redisfc.NewLimiter(clients.RedisClient, cfg.Key, cfg.FixedWindowCounterParams.Window, cfg.FixedWindowCounterParams.Limit), nil
		case config.Memcache:
			// Example for Memcache (implementation not shown)
			// if clients.MemcacheClient == nil {
			// 	return nil, fmt.Errorf("memcache client is required but not provided for memcache backend for key '%s'", cfg.Key)
			// }
			// return memcachefc.NewLimiter(clients.MemcacheClient, cfg.Key, cfg.FixedWindowCounterParams.Window, cfg.FixedWindowCounterParams.Limit), nil
			return nil, fmt.Errorf("memcache backend not yet implemented for fixed window counter for key '%s'", cfg.Key) // Placeholder
		default:
			return nil, fmt.Errorf("unsupported backend type '%s' for fixed window counter for key '%s'", cfg.Backend, cfg.Key)
		}
	// Add cases for other algorithms here
	default:
		return nil, fmt.Errorf("unsupported algorithm type '%s' for key '%s'", cfg.Algorithm, cfg.Key)
	}
}
