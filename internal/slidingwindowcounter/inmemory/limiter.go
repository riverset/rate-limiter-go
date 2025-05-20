package swinmemory

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

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

// Added key parameter to NewLimiter
func NewLimiter(key string, windowSize time.Duration, limit int64) *limiter {
	log.Printf("SlidingWindowCounter(InMemory): Initialized limiter '%s' with window %s and limit %d", key, windowSize, limit)
	return &limiter{
		key:        key, // Store the key
		counter:    sync.Map{},
		windowSize: windowSize,
		limit:      limit,
	}
}

func (l *limiter) Allow(ctx context.Context, identifier string) (bool, error) {

	tempCounter, _ := l.counter.LoadOrStore(identifier, l.initializeWindowCounter(0))
	currentCounter, ok := tempCounter.(*slidingWindowCounter)
	if !ok {
		err := fmt.Errorf("unexpected state type for identifier %s in in-memory limiter '%s'", identifier, l.key)
		// Added limiter key and identifier to error log
		log.Printf("SlidingWindowCounter(InMemory): Limiter '%s': Error in Allow for identifier '%s': %v", l.key, identifier, err)
		return false, err
	}
	currentCounter.mu.Lock()
	defer currentCounter.mu.Unlock()

	// Check if context is cancelled before proceeding
	select {
	case <-ctx.Done():
		// Added limiter key and identifier to log
		log.Printf("SlidingWindowCounter(InMemory): Limiter '%s': Context cancelled for identifier '%s' during check: %v", l.key, identifier, ctx.Err())
		return false, ctx.Err()
	default:
		// Continue
	}

	timeSinceWindowStart := time.Since(currentCounter.currentWindowStart)

	if timeSinceWindowStart >= l.windowSize {
		// log.Printf("SlidingWindowCounter(InMemory): Limiter '%s': Identifier '%s': Window shift detected. Time since start: %s, Window size: %s", l.key, identifier, timeSinceWindowStart, l.windowSize) // Optional: verbose log
		if timeSinceWindowStart < 2*l.windowSize {
			currentCounter.previousWindowCount = currentCounter.currentWindowCount
			currentCounter.currentWindowCount = 0
			// log.Printf("SlidingWindowCounter(InMemory): Limiter '%s': Identifier '%s': Shifted to new window. Previous count: %d, Current count: %d", l.key, identifier, currentCounter.previousWindowCount, currentCounter.currentWindowCount) // Optional: verbose log
		} else {
			currentCounter.currentWindowCount = 0
			currentCounter.previousWindowCount = 0
			// log.Printf("SlidingWindowCounter(InMemory): Limiter '%s': Identifier '%s': Skipped multiple windows. Resetting counts.", l.key, identifier) // Optional: verbose log
		}
		currentCounter.currentWindowStart = time.Now().Truncate(l.windowSize)
		timeSinceWindowStart = time.Since(currentCounter.currentWindowStart) // Recalculate after truncating
	}

	weightCurrentWindow := timeSinceWindowStart.Seconds() / l.windowSize.Seconds()
	weightPreviousWindow := 1 - weightCurrentWindow
	totalRequests := weightCurrentWindow*(float64(currentCounter.currentWindowCount)) + weightPreviousWindow*(float64(currentCounter.previousWindowCount))

	// log.Printf("SlidingWindowCounter(InMemory): Limiter '%s': Identifier '%s': Weighted count %.2f, Limit %d", l.key, identifier, totalRequests, l.limit) // Optional: verbose log

	if totalRequests < float64(l.limit) {
		currentCounter.currentWindowCount += 1
		// log.Printf("SlidingWindowCounter(InMemory): Limiter '%s': Identifier '%s' allowed. New current count: %d", l.key, identifier, currentCounter.currentWindowCount) // Optional: verbose log
		return true, nil
	}

	// log.Printf("SlidingWindowCounter(InMemory): Limiter '%s': Identifier '%s' denied. Weighted count %.2f >= Limit %d", l.key, identifier, totalRequests, l.limit) // Optional: verbose log
	return false, nil

}

func (l *limiter) initializeWindowCounter(previousWindowCount int) *slidingWindowCounter {
	return &slidingWindowCounter{
		previousWindowCount: previousWindowCount,
		currentWindowCount:  0, // Initialize current count to 0 before incrementing in Allow
		currentWindowStart:  time.Now().Truncate(l.windowSize),
	}
}
