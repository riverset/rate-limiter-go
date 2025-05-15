package api

import (
	"learn.ratelimiter/internal/fixedcounter"
	"learn.ratelimiter/internal/tokenbucket"
)

func NewTokenBucketLimiter(rate, capacity int) Limiter {
	return tokenbucket.New(rate, capacity)
}

func NewFixedCounterLimiter(windowSize, limit int) Limiter {
	return fixedcounter.New(windowSize, limit)
}
