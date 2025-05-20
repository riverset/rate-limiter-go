// Package api provides the main interface for initializing and using the rate limiters.
package api

import (
	"fmt"
	"log"

	"learn.ratelimiter/config"
	"learn.ratelimiter/internal/factory"
	"learn.ratelimiter/types"
)

// NewLimiterFactory returns a concrete LimiterFactory based on the algorithm specified in the configuration.
// It takes a LimiterConfig and returns the appropriate factory or an error if the algorithm is unsupported.
func NewLimiterFactory(cfg config.LimiterConfig) (LimiterFactory, error) {
	log.Printf("Factory: Attempting to get factory for algorithm '%s' for limiter key '%s'", cfg.Algorithm, cfg.Key)
	switch cfg.Algorithm {
	case config.FixedWindowCounter:
		return factory.NewFixedWindowFactory()
	case config.SlidingWindowCounter:
		return factory.NewSlidingWindowCounterFactory()
	case config.TokenBucket:
		return factory.NewTokenBucketFactory()
	default:
		err := fmt.Errorf("unsupported algorithm type '%s' for key '%s'", cfg.Algorithm, cfg.Key)
		log.Printf("Factory: Failed to get factory for limiter key '%s': %v", cfg.Key, err)
		return nil, err
	}
}

// LimiterFactory is an interface for creating a Limiter.
type LimiterFactory interface {
	// CreateLimiter creates a new Limiter instance based on the provided configuration and backend clients.
	// It takes a LimiterConfig and BackendClients and returns a Limiter or an error.
	CreateLimiter(cfg config.LimiterConfig, clients types.BackendClients) (types.Limiter, error)
}
