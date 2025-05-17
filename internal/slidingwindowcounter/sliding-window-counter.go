package slidingwindowcounter

import (
	"log"
	"sync"
	"time"
)

type limiter struct {
	counter           sync.Map
	windowSizeSeconds int
	limit             int
}

type slidingWindowCounter struct {
	previousWindowCount int
	currentWindowCount  int
	currentWindowStart  time.Time
	mu                  sync.Mutex
}

func New(windowSizeSeconds, limit int) *limiter {
	return &limiter{
		counter:           sync.Map{},
		windowSizeSeconds: windowSizeSeconds,
		limit:             limit,
	}
}

func (l *limiter) Allow(identifier string) bool {

	tempCounter, _ := l.counter.LoadOrStore(identifier, l.initializeWindowCounter(0))
	currentCounter, ok := tempCounter.(*slidingWindowCounter)
	if !ok {
		log.Printf("Could not convert loaded counter to sliding window counter")
		return false
	}
	currentCounter.mu.Lock()
	defer currentCounter.mu.Unlock()

	timeSinceWindowStart := time.Since(currentCounter.currentWindowStart).Seconds()

	if timeSinceWindowStart >= float64(l.windowSizeSeconds) {
		var newCounter *slidingWindowCounter
		if timeSinceWindowStart < 2*float64(l.windowSizeSeconds) {
			newCounter = l.initializeWindowCounter(currentCounter.currentWindowCount)
		} else {
			newCounter = l.initializeWindowCounter(0)
		}
		currentCounter.mu.Unlock()
		currentCounter = newCounter
		currentCounter.mu.Lock()
		defer currentCounter.mu.Unlock()

		l.counter.Store(identifier, newCounter)
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
