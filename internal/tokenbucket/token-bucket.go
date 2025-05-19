package tokenbucket

import (
	"math"
	"sync"
	"time"
)

type limiter struct {
	buckets  map[string]*tokenBucket
	rate     int
	capacity int
	mu       sync.Mutex
}

type tokenBucket struct {
	tokens     int
	capacity   int
	lastRefill time.Time
}

func New(rate, capacity int) *limiter {
	return &limiter{
		buckets:  make(map[string]*tokenBucket),
		rate:     rate,
		capacity: capacity,
	}
}

func (l *limiter) Allow(identifier string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	bucket, exists := l.buckets[identifier]
	if !exists {
		l.buckets[identifier] = &tokenBucket{
			tokens:     l.capacity,
			capacity:   l.capacity,
			lastRefill: time.Now(),
		}
		bucket = l.buckets[identifier]
	}

	numTokensAdded := int(math.Floor(time.Since(bucket.lastRefill).Seconds() * float64(l.rate)))
	if numTokensAdded > 0 {
		bucket.tokens = min(bucket.capacity, bucket.tokens+int(numTokensAdded))
		bucket.lastRefill = time.Now()
	}

	if bucket.tokens > 0 {
		bucket.tokens -= 1
		return true
	}

	return false
}
