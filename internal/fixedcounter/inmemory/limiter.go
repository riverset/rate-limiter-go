package fcinmemory

import (
	"context"
	"fmt"
	"log" // Added log import
	"sync"
	"time"
)

// CounterState holds the state for a single identifier's counter.
type CounterState struct {
	mu        sync.Mutex
	Count     int64
	WindowEnd time.Time
}

// Limiter implements the Fixed Window Counter algorithm using in-memory storage.
type Limiter struct {
	key    string
	window time.Duration
	limit  int64

	counters sync.Map // Map identifier (e.g., user ID, IP) to *CounterState
}

// NewLimiter creates a new in-memory Fixed Window Counter limiter.
func NewLimiter(key string, window time.Duration, limit int64) *Limiter {
	log.Printf("Initialized in-memory Fixed Window Counter limiter for key '%s' with window %s and limit %d", key, window, limit)
	return &Limiter{
		key:      key,
		window:   window,
		limit:    limit,
		counters: sync.Map{},
	}
}

// Allow checks if a request for the given identifier is allowed.
// It now accepts a context.Context parameter.
func (l *Limiter) Allow(ctx context.Context, identifier string) (bool, error) {
	stateIface, _ := l.counters.LoadOrStore(identifier, &CounterState{})

	state, ok := stateIface.(*CounterState)
	if !ok {
		err := fmt.Errorf("unexpected state type for identifier %s in in-memory limiter '%s'", identifier, l.key)
		log.Printf("Error in Allow for key '%s', identifier '%s': %v", l.key, identifier, err)
		return false, err
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	now := time.Now()

	if now.After(state.WindowEnd) {
		state.Count = 0
		state.WindowEnd = now.Add(l.window)
	}

	if state.Count < l.limit {
		state.Count++
		return true, nil
	}

	return false, nil
}
