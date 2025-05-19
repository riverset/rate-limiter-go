package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync/atomic" // Import atomic for reading metrics

	"learn.ratelimiter/metrics"
	"learn.ratelimiter/middleware"
)

// Define a struct for the application dependencies needed in main
type application struct {
	RateLimiterMiddleware *middleware.RateLimitMiddleware
	Metrics               *metrics.RateLimitMetrics
}

func main() {
	// Ensure you have run 'go get github.com/go-redis/redis/v8 gopkg.in/yaml.v2 github.com/google/wire/cmd/wire'
	// And run 'wire generate ./...' after making changes to wire.go

	// Call the Wire-generated injector to build the application dependencies
	app, err := InitializeApplication()
	if err != nil {
		log.Fatalf("Error initializing application with Wire: %v", err)
	}
	log.Println("Application components initialized with Wire")

	http.HandleFunc("/unlimited", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK) // This sets 200 status code
		fmt.Fprintln(w, "Unlimited! Let's Go!")
	})

	// Use the middleware from the injected application struct
	http.HandleFunc("/limited", app.RateLimiterMiddleware.Handle(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK) // This sets 200 status code
		fmt.Fprintln(w, "Limited, don't over use me!")
	}, getClientIP))

	// Add metrics endpoint - reading public fields from the metrics struct using atomic loads
	// Use the metrics instance from the injected application struct
	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		// Use atomic.LoadInt32 for thread-safe reading of counters updated with atomic.AddInt32
		fmt.Fprintf(w, "Total Requests: %d\n", atomic.LoadInt32(&app.Metrics.TotalRequests))
		fmt.Fprintf(w, "Allowed Requests: %d\n", atomic.LoadInt32(&app.Metrics.AllowedRequests))
		fmt.Fprintf(w, "Rejected Requests: %d\n", atomic.LoadInt32(&app.Metrics.RejectedRequests))
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
