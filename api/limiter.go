package api

type Limiter interface {
	Allow(identifier string) bool
}
