package fixedcounter

import (
	"sync"
	"time"
)

type limiter struct {
	counters   map[string]*windowCounter
	windowSize int
	limit      int
	mu         sync.Mutex
}

type windowCounter struct {
	requests    int
	windowStart time.Time
}

func New(windowSize, limit int) *limiter {

	return &limiter{
		counters:   make(map[string]*windowCounter),
		windowSize: windowSize,
		limit:      limit,
	}
}

func (l *limiter) Allow(identifier string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	counter, existsOk := l.counters[identifier]
	if !existsOk {
		l.counters[identifier] = l.resetCounter()
		return true
	}

	timeDifference := time.Since(counter.windowStart)
	if timeDifference.Seconds() >= float64(l.windowSize) {
		l.counters[identifier] = l.resetCounter()
		return true
	}
	if counter.requests < l.limit {
		counter.requests += 1
		return true
	}
	return false

}

func (l *limiter) resetCounter() *windowCounter {
	return &windowCounter{
		requests:    1,
		windowStart: time.Now().Truncate(time.Second * time.Duration(l.windowSize)),
	}
}
