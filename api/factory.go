package api

import (
	"fmt"
	"log" // Added log import

	"learn.ratelimiter/config"
	"learn.ratelimiter/internal/factory"
	"learn.ratelimiter/types"
)

// NewFactory returns a concrete LimiterFactory based on the algorithm.
func NewFactory(cfg config.LimiterConfig) (LimiterFactory, error) {
	log.Printf("Getting factory for algorithm '%s'", cfg.Algorithm)
	switch cfg.Algorithm {
	case config.FixedWindowCounter:
		return factory.NewFixedWindowFactory(), nil
	default:
		err := fmt.Errorf("unsupported algorithm type '%s' for key '%s'", cfg.Algorithm, cfg.Key)
		log.Printf("Failed to get factory: %v", err)
		return nil, err
	}
}

// LimiterFactory is an interface for creating a Limiter.
type LimiterFactory interface {
	CreateLimiter(cfg config.LimiterConfig, clients types.BackendClients) (types.Limiter, error)
}
