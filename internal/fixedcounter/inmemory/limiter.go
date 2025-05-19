package inmemory

import (
	"fmt"
	"sync"
	"time"
)

// CounterState holds the state for a single identifier's counter.
// It now includes a mutex to protect its internal fields.
type CounterState struct {
	mu        sync.Mutex // Mutex to protect Count and WindowEnd
	Count     int64
	WindowEnd time.Time
}

// Limiter implements the Fixed Window Counter algorithm using in-memory storage.
type Limiter struct {
	key    string
	window time.Duration
	limit  int64

	// Use sync.Map for concurrent-safe map operations
	counters sync.Map // Map identifier (e.g., user ID, IP) to *CounterState
}

// NewLimiter creates a new in-memory Fixed Window Counter limiter.
func NewLimiter(key string, window time.Duration, limit int64) *Limiter {
	return &Limiter{
		key:      key, // This key might be used as a prefix if storing multiple limiters in one map
		window:   window,
		limit:    limit,
		counters: sync.Map{}, // Initialize sync.Map
	}
}

// Allow checks if a request for the given identifier is allowed.
func (l *Limiter) Allow(identifier string) (bool, error) {
	// Use LoadOrStore to get the existing state or store a new one atomically for map access
	stateIface, _ := l.counters.LoadOrStore(identifier, &CounterState{})

	// Assert the type to our CounterState struct pointer
	state, ok := stateIface.(*CounterState)
	if !ok {
		// This should ideally not happen if only *CounterState is stored
		// Handle unexpected type if necessary, though unlikely in this controlled use
		return false, fmt.Errorf("unexpected state type for identifier %s", identifier)
	}

	// Lock the mutex specific to this identifier's state
	state.mu.Lock()
	defer state.mu.Unlock() // Ensure the mutex is unlocked when the function exits

	now := time.Now()

	// If the window has ended or doesn't exist (first access), reset the counter and start a new window
	if now.After(state.WindowEnd) {
		state.Count = 0
		state.WindowEnd = now.Add(l.window)
	}

	// Check if the current count is within the limit
	if state.Count < l.limit {
		state.Count++
		return true, nil
	}

	// Limit exceeded
	return false, nil
}
