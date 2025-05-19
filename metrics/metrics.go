package metrics

// import "sync/atomic" // Import atomic

// // RateLimitMetrics holds counters for rate limiting statistics.
// type RateLimitMetrics struct {
// 	TotalRequests   int64
// 	AllowedRequests int64
// 	RejectedRequests int64
// }

// // NewRateLimitMetrics creates a new RateLimitMetrics instance.
// func NewRateLimitMetrics() *RateLimitMetrics {
// 	return &RateLimitMetrics{}
// }

// // IncrementTotal increments the total requests counter.
// func (m *RateLimitMetrics) IncrementTotal() {
// 	atomic.AddInt64(&m.TotalRequests, 1)
// }

// // IncrementAllowed increments the allowed requests counter.
// func (m *RateLimitMetrics) IncrementAllowed() {
// 	atomic.AddInt64(&m.AllowedRequests, 1)
// }

// // IncrementRejected increments the rejected requests counter.
// func (m *RateLimitMetrics) IncrementRejected() {
// 	atomic.AddInt64(&m.RejectedRequests, 1)
// }
