package swinmemory

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

type limiter struct {
	key        string
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

func NewLimiter(key string, windowSize time.Duration, limit int64) *limiter {
	log.Printf("Initialized in-memory Sliding Window Counter limiter for key '%s' with window %s and limit %d", key, windowSize, limit)
	return &limiter{
		key:        key,
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
		log.Printf("Error in Allow for key '%s', identifier '%s': %v", l.key, identifier, err)
		return false, err
	}
	currentCounter.mu.Lock()
	defer currentCounter.mu.Unlock()

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
		timeSinceWindowStart = time.Since(currentCounter.currentWindowStart)
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
		currentWindowCount:  1,
		currentWindowStart:  time.Now().Truncate(l.windowSize),
	}
}
