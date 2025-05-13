package tokenbucket

import (
	"sync"
	"time"
)

type tokenBucket struct {
	tokens     int
	capacity   int
	lastRefill time.Time
}

type limiter struct {
	buckets  map[string]*tokenBucket
	mu       sync.Mutex
	rate     int
	capacity int
}

func New(rate, capacity int) *limiter {
	return &limiter{
		buckets:  make(map[string]*tokenBucket),
		rate:     rate,
		capacity: capacity,
	}
}

func (*limiter) Allow(identifier string) bool {
	return true
}
