package api

import (
	"fmt"

	"learn.ratelimiter/config"
	"learn.ratelimiter/internal/factory"
	"learn.ratelimiter/types"
)

// NewFactory returns a concrete LimiterFactory based on the algorithm.
func NewFactory(cfg config.LimiterConfig) (LimiterFactory, error) {
	switch cfg.Algorithm {
	case config.FixedWindowCounter:
		return factory.NewFixedWindowFactory(), nil
	default:
		return nil, fmt.Errorf("unsupported algorithm type '%s' for key '%s'", cfg.Algorithm, cfg.Key)
	}
}

// LimiterFactory is an interface for creating a Limiter.
type LimiterFactory interface {
	CreateLimiter(cfg config.LimiterConfig, clients types.BackendClients) (types.Limiter, error)
}
