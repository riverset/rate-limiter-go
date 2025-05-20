// Package swinmemory provides an in-memory implementation of the Sliding Window Counter rate limiting algorithm.
package swinmemory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log" // Import zerolog's global logger
)

// limiter is the in-memory implementation of the Sliding Window Counter.
// It stores the counts for each identifier in a sync.Map.
type limiter struct {
	key        string // Limiter key from config
	counter    sync.Map
	windowSize time.Duration
	limit      int64
}

type slidingWindowCounter struct {
	previousWindowCount int
	currentWindowCount  int
	currentWindowStart  time.Time
	mu                  sync.Mutex
}

// NewLimiter creates a new in-memory Sliding Window Counter limiter.
// It takes a unique key for the limiter, the size of the sliding window, and the maximum limit of requests within the window.
func NewLimiter(key string, windowSize time.Duration, limit int64) *limiter {
	log.Info().Str("limiter_type", "SlidingWindowCounter").Str("backend", "InMemory").Str("limiter_key", key).Dur("window", windowSize).Int64("limit", limit).Msg("Limiter: Initialized")
	return &limiter{
		key:        key, // Store the key
		counter:    sync.Map{},
		windowSize: windowSize,
		limit:      limit,
	}
}

// Allow checks if a request is allowed for the given identifier based on the Sliding Window Counter algorithm.
// It takes a context and an identifier and returns true if the request is allowed, false otherwise, and an error if any occurred.
func (l *limiter) Allow(ctx context.Context, identifier string) (bool, error) {

	tempCounter, _ := l.counter.LoadOrStore(identifier, l.initializeWindowCounter(0))
	currentCounter, ok := tempCounter.(*slidingWindowCounter)
	if !ok {
		err := fmt.Errorf("unexpected state type for identifier %s in in-memory limiter '%s'", identifier, l.key)
		// Added limiter key and identifier to error log
		log.Error().Err(err).Str("limiter_type", "SlidingWindowCounter").Str("backend", "InMemory").Str("limiter_key", l.key).Str("identifier", identifier).Msg("Limiter: Error in Allow")
		return false, err
	}
	currentCounter.mu.Lock()
	defer currentCounter.mu.Unlock()

	// Check if context is cancelled before proceeding
	select {
	case <-ctx.Done():
		// Added limiter key and identifier to log
		log.Warn().Err(ctx.Err()).Str("limiter_type", "SlidingWindowCounter").Str("backend", "InMemory").Str("limiter_key", l.key).Str("identifier", identifier).Msg("Limiter: Context cancelled during check")
		return false, ctx.Err()
	default:
		// Continue
	}

	timeSinceWindowStart := time.Since(currentCounter.currentWindowStart)

	if timeSinceWindowStart >= l.windowSize {
		if timeSinceWindowStart < 2*l.windowSize {
			currentCounter.previousWindowCount = currentCounter.currentWindowCount
			currentCounter.currentWindowCount = 0
		} else {
			currentCounter.currentWindowCount = 0
			currentCounter.previousWindowCount = 0
		}
		currentCounter.currentWindowStart = time.Now().Truncate(l.windowSize)
		timeSinceWindowStart = time.Since(currentCounter.currentWindowStart) // Recalculate after truncating
	}

	weightCurrentWindow := timeSinceWindowStart.Seconds() / l.windowSize.Seconds()
	weightPreviousWindow := 1 - weightCurrentWindow
	totalRequests := weightCurrentWindow*(float64(currentCounter.currentWindowCount)) + weightPreviousWindow*(float64(currentCounter.previousWindowCount))

	if totalRequests < float64(l.limit) {
		currentCounter.currentWindowCount += 1
		return true, nil
	}

	return false, nil

}

func (l *limiter) initializeWindowCounter(previousWindowCount int) *slidingWindowCounter {
	return &slidingWindowCounter{
		previousWindowCount: previousWindowCount,
		currentWindowCount:  0, // Initialize current count to 0 before incrementing in Allow
		currentWindowStart:  time.Now().Truncate(l.windowSize),
	}
}
