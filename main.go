package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync/atomic"

	ratelimiter "learn.ratelimiter/api"
	"learn.ratelimiter/metrics"
	"learn.ratelimiter/middleware"
)

func main() {
	rateLimiter, err := ratelimiter.NewLimiterFromConfigPath("config.yaml")
	if err != nil {
		log.Fatalf("Error initializing rate limiter from config: %v", err)
	}

	log.Println("Rate limiter successfully initialized from config.")

	metrics := metrics.NewRateLimitMetrics()

	rateLimitMiddleware := middleware.NewRateLimitMiddleware(rateLimiter, metrics)

	http.HandleFunc("/unlimited", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "Unlimited! Let's Go!")
	})

	http.HandleFunc("/limited", rateLimitMiddleware.Handle(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "Limited, don't over use me!")
	}, getClientIP))

	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Total Requests: %d\n", atomic.LoadInt32(&metrics.TotalRequests))
		fmt.Fprintf(w, "Allowed Requests: %d\n", atomic.LoadInt32(&metrics.AllowedRequests))
		fmt.Fprintf(w, "Rejected Requests: %d\n", atomic.LoadInt32(&metrics.RejectedRequests))
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
