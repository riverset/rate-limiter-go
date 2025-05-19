package api

// Limiter is the interface that all rate limiting algorithms must implement.
type Limiter interface {
	Allow(key string) (bool, error)
}
