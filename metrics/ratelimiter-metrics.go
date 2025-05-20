// Package metrics contains code related to metrics and monitoring for the rate limiter.
package metrics

import (
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// RateLimitMetrics keeps track of rate limiting statistics.
type RateLimitMetrics struct {
	// TotalRequests is the total number of requests processed by the rate limiter.
	TotalRequests int32
	// RejectedRequests is the number of requests that were rejected due to rate limiting.
	RejectedRequests int32
	// AllowedRequests is the number of requests that were allowed by the rate limiter.
	AllowedRequests int32

	// Prometheus metrics
	allowedRequests  *prometheus.CounterVec
	rejectedRequests *prometheus.CounterVec
}

// NewRateLimitMetrics creates a new instance of RateLimitMetrics.
func NewRateLimitMetrics() *RateLimitMetrics {
	metrics := &RateLimitMetrics{
		allowedRequests: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rate_limiter_allowed_requests_total",
				Help: "Total number of requests allowed by the rate limiter.",
			},
			[]string{"limiter_key", "algorithm"},
		),
		rejectedRequests: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rate_limiter_rejected_requests_total",
				Help: "Total number of requests rejected by the rate limiter.",
			},
			[]string{"limiter_key", "algorithm"},
		),
	}
	return metrics
}

// RecordRequest updates the metrics based on whether the request was allowed or rejected.
// It takes a boolean indicating if the request was allowed.
// This function is now deprecated in favor of RecordRequestWithLabels.
func (r *RateLimitMetrics) RecordRequest(allowed bool) {
	atomic.AddInt32(&r.TotalRequests, 1)
	if allowed {
		atomic.AddInt32(&r.AllowedRequests, 1)
	} else {
		atomic.AddInt32(&r.RejectedRequests, 1)
	}
}

// RecordRequestWithLabels updates the Prometheus metrics with labels.
func (r *RateLimitMetrics) RecordRequestWithLabels(allowed bool, limiterKey, algorithm string) {
	if allowed {
		r.allowedRequests.WithLabelValues(limiterKey, algorithm).Inc()
	} else {
		r.rejectedRequests.WithLabelValues(limiterKey, algorithm).Inc()
	}
}
