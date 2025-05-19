package inmemory

import (
	"context"
	"fmt"
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
		return false, fmt.Errorf("unexpected state type for identifier %s", identifier)
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
