// Package fcinmemory provides an in-memory implementation of the Fixed Window Counter rate limiting algorithm.
package fcinmemory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log" // Import zerolog's global logger
)

// CounterState holds the state for a single identifier's counter.
type CounterState struct {
	mu        sync.Mutex
	Count     int64
	WindowEnd time.Time
}

// Limiter implements the Fixed Window Counter algorithm using in-memory storage.
// It stores the counts for each identifier in a sync.Map.
type Limiter struct {
	key    string // Limiter key from config
	window time.Duration
	limit  int64

	counters sync.Map // Map identifier (e.g., user ID, IP) to *CounterState
}

// NewLimiter creates a new in-memory Fixed Window Counter limiter.
// It takes a unique key for the limiter, the size of the window, and the maximum limit of requests within the window.
func NewLimiter(key string, window time.Duration, limit int64) *Limiter {
	log.Info().Str("limiter_type", "FixedWindowCounter").Str("backend", "InMemory").Str("limiter_key", key).Dur("window", window).Int64("limit", limit).Msg("Limiter: Initialized")
	return &Limiter{
		key:      key,
		window:   window,
		limit:    limit,
		counters: sync.Map{},
	}
}

// Allow checks if a request for the given identifier is allowed based on the Fixed Window Counter algorithm.
// It takes a context and an identifier and returns true if the request is allowed, false otherwise, and an error if any occurred.
func (l *Limiter) Allow(ctx context.Context, identifier string) (bool, error) {
	stateIface, _ := l.counters.LoadOrStore(identifier, &CounterState{})

	state, ok := stateIface.(*CounterState)
	if !ok {
		err := fmt.Errorf("unexpected state type for identifier %s in in-memory limiter '%s'", identifier, l.key)
		// Added limiter key and identifier to error log
		log.Error().Err(err).Str("limiter_type", "FixedWindowCounter").Str("backend", "InMemory").Str("limiter_key", l.key).Str("identifier", identifier).Msg("Limiter: Error in Allow")
		return false, err
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	now := time.Now()

	// Check if context is cancelled before proceeding
	select {
	case <-ctx.Done():
		// Added limiter key and identifier to log
		log.Warn().Err(ctx.Err()).Str("limiter_type", "FixedWindowCounter").Str("backend", "InMemory").Str("limiter_key", l.key).Str("identifier", identifier).Msg("Limiter: Context cancelled during check")
		return false, ctx.Err()
	default:
		// Continue
	}

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
