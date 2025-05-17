package ratelimiter

type Limiter interface {
	Allow(identifier string) bool
}
