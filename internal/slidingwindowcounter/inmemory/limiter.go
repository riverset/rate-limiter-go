package inmemory

import (
	"log"
	"sync"
	"time"
)

type limiter struct {
	key        string
	counter    sync.Map
	windowSize time.Duration
	limit      int
}

type slidingWindowCounter struct {
	previousWindowCount int
	currentWindowCount  int
	currentWindowStart  time.Time
	mu                  sync.Mutex
}

func NewLimiter(key string, windowSize time.Duration, limit int) *limiter {
	log.Printf("Initialized in-memory Sliding Window Counter limiter for key '%s' with window %s and limit %d", key, windowSize, limit)
	return &limiter{
		key:        key,
		counter:    sync.Map{},
		windowSize: windowSize,
		limit:      limit,
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
		return true
	}

	return false

}

func (l *limiter) initializeWindowCounter(previousWindowCount int) *slidingWindowCounter {
	return &slidingWindowCounter{
		previousWindowCount: previousWindowCount,
		currentWindowCount:  1,
		currentWindowStart:  time.Now().Truncate(l.windowSize),
	}
}
