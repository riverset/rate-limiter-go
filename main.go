package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"

	ratelimiter "learn.ratelimiter/api"
	"learn.ratelimiter/metrics"
	"learn.ratelimiter/middleware"
)

func main() {

	// Initialize limiter
	rateLimiter := ratelimiter.NewSlidingWindowCounter(1, 2)

	// Initialize metrics
	metrics := metrics.NewRateLimitMetrics()

	// Initialize middleware
	rateLimitMiddleware := middleware.NewRateLimitMiddleware(rateLimiter, metrics)

	http.HandleFunc("/unlimited", func(w http.ResponseWriter, r *http.Request) {

		w.WriteHeader(http.StatusOK) // This sets 200 status code
		fmt.Fprintln(w, "Unlimited! Let's Go!")
	})

	http.HandleFunc("/limited", rateLimitMiddleware.Handle(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK) // This sets 200 status code
		fmt.Fprintln(w, "Limited, don't over use me!")
	}, getClientIP))

	// Add metrics endpoint
	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Total Requests: %d\n", metrics.TotalRequests)
		fmt.Fprintf(w, "Allowed Requests: %d\n", metrics.AllowedRequests)
		fmt.Fprintf(w, "Rejected Requests: %d\n", metrics.RejectedRequests)
	})

	log.Println("Starting server on :8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func getClientIP(r *http.Request) string {
	ip := r.Header.Get("X-Forwarded-For")
	if ip != "" {
		return strings.Split(ip, ",")[0]
	}

	ip = r.Header.Get("X-Real-IP")
	if ip != "" {
		return ip
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
