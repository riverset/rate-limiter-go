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
type Limiter struct {
	key    string // Limiter key from config
	window time.Duration
	limit  int64

	counters sync.Map // Map identifier (e.g., user ID, IP) to *CounterState
}

// NewLimiter creates a new in-memory Fixed Window Counter limiter.
func NewLimiter(key string, window time.Duration, limit int64) *Limiter {
	log.Info().Str("limiter_type", "FixedWindowCounter").Str("backend", "InMemory").Str("limiter_key", key).Dur("window", window).Int64("limit", limit).Msg("Limiter: Initialized")
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
