package fcinmemory

import (
	"context"
	"fmt"
	"log"
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
	key    string // Limiter key from config
	window time.Duration
	limit  int64

	counters sync.Map // Map identifier (e.g., user ID, IP) to *CounterState
}

// NewLimiter creates a new in-memory Fixed Window Counter limiter.
func NewLimiter(key string, window time.Duration, limit int64) *Limiter {
	log.Printf("FixedWindowCounter(InMemory): Initialized limiter '%s' with window %s and limit %d", key, window, limit)
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
		// Added limiter key and identifier to error log
		log.Printf("FixedWindowCounter(InMemory): Limiter '%s': Error in Allow for identifier '%s': %v", l.key, identifier, err)
		return false, err
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	now := time.Now()

	// Check if context is cancelled before proceeding
	select {
	case <-ctx.Done():
		// Added limiter key and identifier to log
		log.Printf("FixedWindowCounter(InMemory): Limiter '%s': Context cancelled for identifier '%s' during check: %v", l.key, identifier, ctx.Err())
		return false, ctx.Err()
	default:
		// Continue
	}

	if now.After(state.WindowEnd) {
		// log.Printf("FixedWindowCounter(InMemory): Limiter '%s': Identifier '%s': Window reset. Old end: %s, New end: %s", l.key, identifier, state.WindowEnd, now.Add(l.window)) // Optional: verbose log
		state.Count = 0
		state.WindowEnd = now.Add(l.window)
	}

	// log.Printf("FixedWindowCounter(InMemory): Limiter '%s': Identifier '%s': Current count %d, Limit %d", l.key, identifier, state.Count, l.limit) // Optional: verbose log

	if state.Count < l.limit {
		state.Count++
		// log.Printf("FixedWindowCounter(InMemory): Limiter '%s': Identifier '%s' allowed. New count: %d", l.key, identifier, state.Count) // Optional: verbose log
		return true, nil
	}

	// log.Printf("FixedWindowCounter(InMemory): Limiter '%s': Identifier '%s' denied. Count %d >= Limit %d", l.key, identifier, state.Count, l.limit) // Optional: verbose log
	return false, nil
}
