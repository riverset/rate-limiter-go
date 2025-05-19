package api

// Limiter is the interface that all rate limiting algorithms must implement.
type Limiter interface {
	// Allow checks if a request identified by 'key' is allowed.
	// It returns true if allowed, false otherwise, and an error if something went wrong.
	Allow(key string) (bool, error)
}
