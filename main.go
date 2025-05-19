package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync/atomic" // Import atomic for reading metrics

	ratelimiter "learn.ratelimiter/api" // Import the api package
	"learn.ratelimiter/metrics"
	"learn.ratelimiter/middleware"
)

func main() {
	// Ensure you have run 'go get github.com/go-redis/redis/v8 gopkg.in/yaml.v2'
	// Wire is no longer needed for the consumer's main.go

	// Use the public API function to create the limiter from config
	rateLimiter, err := ratelimiter.NewLimiterFromConfigPath("config.yaml")
	if err != nil {
		log.Fatalf("Error initializing rate limiter from config: %v", err)
	}

	log.Println("Rate limiter successfully initialized from config.")

	// Initialize metrics (still done in main as it's application-specific)
	metrics := metrics.NewRateLimitMetrics()

	// Initialize middleware using the created limiter and metrics
	rateLimitMiddleware := middleware.NewRateLimitMiddleware(rateLimiter, metrics)

	http.HandleFunc("/unlimited", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK) // This sets 200 status code
		fmt.Fprintln(w, "Unlimited! Let's Go!")
	})

	// Use the middleware
	http.HandleFunc("/limited", rateLimitMiddleware.Handle(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK) // This sets 200 status code
		fmt.Fprintln(w, "Limited, don't over use me!")
	}, getClientIP))

	// Add metrics endpoint - reading public fields from the metrics struct using atomic loads
	// Use the metrics instance created in main
	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		// Use atomic.LoadInt32 for thread-safe reading of counters updated with atomic.AddInt32
		fmt.Fprintf(w, "Total Requests: %d\n", atomic.LoadInt32(&metrics.TotalRequests))
		fmt.Fprintf(w, "Allowed Requests: %d\n", atomic.LoadInt32(&metrics.AllowedRequests))
		fmt.Fprintf(w, "Rejected Requests: %d\n", atomic.LoadInt32(&metrics.RejectedRequests))
	})

	log.Println("Starting server on :8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))

	// Note: If backend clients were initialized within NewLimiterFromConfigPath
	// and need graceful shutdown, the Limiter interface or the NewLimiterFromConfigPath
	// function would need to provide a way to access/close them.
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
