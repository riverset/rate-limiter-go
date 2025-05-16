package slidingwindowcounter

import (
	"sync"
	"time"
)

type limiter struct {
	counter           map[string]*slidingWindowCounter
	windowSizeSeconds int
	limit             int
	mu                sync.Mutex
}

type slidingWindowCounter struct {
	previousWindowCount int
	currentWindowCount  int
	currentWindowStart  time.Time
}

func New(windowSizeSeconds, limit int) *limiter {
	return &limiter{
		counter:           make(map[string]*slidingWindowCounter),
		windowSizeSeconds: windowSizeSeconds,
		limit:             limit,
	}
}

func (l *limiter) Allow(identifier string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	currentCounter, exists := l.counter[identifier]
	if !exists {
		l.counter[identifier] = l.initializeWindowCounter(0)
		return true
	}

	timeSinceWindowStart := time.Since(currentCounter.currentWindowStart).Seconds()

	if timeSinceWindowStart >= float64(l.windowSizeSeconds) {
		if timeSinceWindowStart < 2*float64(l.windowSizeSeconds) {
			currentCounter = l.initializeWindowCounter(currentCounter.currentWindowCount)
		} else {
			currentCounter = l.initializeWindowCounter(0)
		}
		l.counter[identifier] = currentCounter
		timeSinceWindowStart = time.Since(currentCounter.currentWindowStart).Seconds()
	}

	weightCurrentWindow := timeSinceWindowStart / float64(l.windowSizeSeconds)
	weightPreviousWindow := 1 - weightCurrentWindow
	totalRequests := weightCurrentWindow*(float64(currentCounter.currentWindowCount)) + weightPreviousWindow*(float64(currentCounter.previousWindowCount))

	if totalRequests < float64(l.limit) {
		currentCounter.currentWindowCount += 1
		return true
	}

	return false

}

func (l *limiter) initializeWindowCounter(previousWindowCount int) *slidingWindowCounter {
	return &slidingWindowCounter{
		previousWindowCount: previousWindowCount,
		currentWindowCount:  1,
		currentWindowStart:  time.Now().Truncate(time.Duration(l.windowSizeSeconds) * time.Second),
	}
}
