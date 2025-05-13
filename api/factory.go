package api

import "learn.ratelimiter/internal/tokenbucket"

func NewTokenBucketLimiter(rate, capacity int) Limiter {
	return tokenbucket.New(rate, capacity)
}
