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
		log.Error().Err(err).Str("limiter_type", "SlidingWindowCounter").Str("backend", "InMemory").Str("limiter_key", l.key).Str("identifier", identifier).Msg("Limiter: Error in Allow")
		return false, err
	}
	currentCounter.mu.Lock()
	defer currentCounter.mu.Unlock()

	// Check if context is cancelled before proceeding
	select {
	case <-ctx.Done():
		log.Warn().Err(ctx.Err()).Str("limiter_type", "SlidingWindowCounter").Str("backend", "InMemory").Str("limiter_key", l.key).Str("identifier", identifier).Msg("Limiter: Context cancelled during check")
		return false, ctx.Err()
	default:
		// Continue
	}

	now := time.Now()

	// Slide the window if necessary
	for now.Sub(currentCounter.currentWindowStart) >= l.windowSize {
		currentCounter.previousWindowCount = currentCounter.currentWindowCount
		currentCounter.currentWindowCount = 0
		currentCounter.currentWindowStart = currentCounter.currentWindowStart.Add(l.windowSize)
		// If the time elapsed is more than twice the window size, reset both counts
		if now.Sub(currentCounter.currentWindowStart) >= l.windowSize {
			currentCounter.previousWindowCount = 0
		}
	}

	// Calculate the weighted total requests in the sliding window [now - windowSize, now]
	// This window overlaps with the previous bucket [currentWindowStart - windowSize, currentWindowStart]
	// and the current bucket [currentWindowStart, now].

	// Time elapsed in the current window [currentWindowStart, now]
	timeInCurrentWindow := now.Sub(currentCounter.currentWindowStart)
	// Percentage of the previous bucket that overlaps with the sliding window
	percentagePreviousOverlap := float64(l.windowSize-timeInCurrentWindow) / float64(l.windowSize)

	// Total requests in the sliding window
	totalRequests := float64(currentCounter.currentWindowCount) + float64(currentCounter.previousWindowCount)*percentagePreviousOverlap

	// Check if allowing the current request would exceed the limit
	if totalRequests+1 <= float64(l.limit) {
		currentCounter.currentWindowCount++
		return true, nil
	}

	return false, nil

}

func (l *limiter) initializeWindowCounter(previousWindowCount int) *slidingWindowCounter {
	return &slidingWindowCounter{
		previousWindowCount: previousWindowCount,
		currentWindowCount:  0,
		currentWindowStart:  time.Now(), // Initial window starts now
	}
}
