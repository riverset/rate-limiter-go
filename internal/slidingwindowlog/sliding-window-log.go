package slidingwindowlog

import (
	"sync"
	"time"

	"github.com/gammazero/deque"
)

type limiter struct {
	logs       map[string]*deque.Deque[time.Time]
	windowSize int
	limit      int
	mu         sync.Mutex
}

func New(windowSizeSeconds, limit int) *limiter {
	return &limiter{
		logs:       make(map[string]*deque.Deque[time.Time]),
		windowSize: windowSizeSeconds,
		limit:      limit,
	}
}

func (l *limiter) Allow(identifier string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	currentLog, exists := l.logs[identifier]
	if !exists {
		var q deque.Deque[time.Time]
		q.PushBack(time.Now())

		l.logs[identifier] = &q
		return true
	}

	// remove logs which are beyond current Window
	for currentLog.Len() > 0 && time.Since(currentLog.Front()).Seconds() > float64(l.windowSize) {
		currentLog.PopFront()
	}

	// get number of logs
	if currentLog.Len() < l.limit {
		currentLog.PushBack(time.Now())
		return true
	}
	return false

}
