package inmemory

import (
	"sync"
	"time"
)

// Limiter implements the Fixed Window Counter algorithm using in-memory storage.
type Limiter struct {
	key    string
	window time.Duration
	limit  int64

	mu        sync.Mutex
	counter   map[string]int64     // Map key (e.g., user ID, IP) to count
	windowEnd map[string]time.Time // Map key to the end time of the current window
}

// NewLimiter creates a new in-memory Fixed Window Counter limiter.
func NewLimiter(key string, window time.Duration, limit int64) *Limiter {
	return &Limiter{
		key:       key, // This key might be used as a prefix if storing multiple limiters in one map
		window:    window,
		limit:     limit,
		counter:   make(map[string]int64),
		windowEnd: make(map[string]time.Time),
	}
}

// Allow checks if a request for the given identifier is allowed.
func (l *Limiter) Allow(identifier string) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	currentWindowEnd, exists := l.windowEnd[identifier]

	// If the window has ended or doesn't exist, reset the counter and start a new window
	if !exists || now.After(currentWindowEnd) {
		l.counter[identifier] = 0
		l.windowEnd[identifier] = now.Add(l.window)
	}

	// Check if the current count is within the limit
	if l.counter[identifier] < l.limit {
		l.counter[identifier]++
		return true, nil
	}

	// Limit exceeded
	return false, nil
}
