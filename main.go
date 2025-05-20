package main

import (
	"flag" // Import the flag package
	"fmt"  // Import io for the Closer interface
	"log"
	"net"
	"net/http"
	"strings"
	"sync/atomic"

	ratelimiter "learn.ratelimiter/api"
	"learn.ratelimiter/metrics"
	"learn.ratelimiter/middleware"
	// Import types to use types.Limiter
)

func main() {
	// Define a flag for the port, defaulting to 8080
	port := flag.Int("p", 8080, "Port to run the HTTP server on")

	// Parse the command-line flags
	flag.Parse()

	// Use the new function to initialize multiple limiters and get the closer
	limiters, closer, err := ratelimiter.NewLimitersFromConfigPath("config.yaml") // Updated variables
	if err != nil {
		log.Fatalf("Error initializing rate limiters from config: %v", err)
	}
	// Defer the Close method on the returned closer
	defer closer.Close()

	log.Println("Rate limiters successfully initialized from config.")

	// Retrieve specific limiters from the map
	apiRateLimiter, ok := limiters["api_rate_limit"] // Access from the returned map
	if !ok {
		log.Fatalf("Rate limiter with key 'api_rate_limit' not found in config")
	}

	userLoginRateLimiter, ok := limiters["user_login_rate_limit_distributed"] // Access from the returned map
	if !ok {
		log.Fatalf("Rate limiter with key 'user_login_rate_limit_distributed' not found in config") // Updated error message
	}

	// You can now use different limiters for different routes or logic
	apiMetrics := metrics.NewRateLimitMetrics()
	userLoginMetrics := metrics.NewRateLimitMetrics() // Example: separate metrics per limiter

	apiRateLimitMiddleware := middleware.NewRateLimitMiddleware(apiRateLimiter, apiMetrics)
	userLoginRateLimitMiddleware := middleware.NewRateLimitMiddleware(userLoginRateLimiter, userLoginMetrics) // Example: separate middleware per limiter

	http.HandleFunc("/unlimited", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "Unlimited! Let's Go!")
	})

	// Apply the 'api_rate_limit' middleware to the /limited route
	http.HandleFunc("/limited", apiRateLimitMiddleware.Handle(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "Limited, don't over use me!")
	}, getClientIP))

	// Example of applying the 'user_login_rate_limit' middleware to another route
	http.HandleFunc("/login", userLoginRateLimitMiddleware.Handle(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "Login attempt processed!")
	}, getClientIP)) // You might use a different identifier func for login (e.g., username)

	// Update metrics endpoint to show metrics for both limiters
	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "--- API Rate Limit Metrics ---")
		fmt.Fprintf(w, "Total Requests: %d\n", atomic.LoadInt32(&apiMetrics.TotalRequests))
		fmt.Fprintf(w, "Allowed Requests: %d\n", atomic.LoadInt32(&apiMetrics.AllowedRequests))
		fmt.Fprintf(w, "Rejected Requests: %d\n", atomic.LoadInt32(&apiMetrics.RejectedRequests))

		fmt.Fprintln(w, "\n--- User Login Rate Limit Metrics ---")
		fmt.Fprintf(w, "Total Requests: %d\n", atomic.LoadInt32(&userLoginMetrics.TotalRequests))
		fmt.Fprintf(w, "Allowed Requests: %d\n", atomic.LoadInt32(&userLoginMetrics.AllowedRequests))
		fmt.Fprintf(w, "Rejected Requests: %d\n", atomic.LoadInt32(&userLoginMetrics.RejectedRequests))
	})

	// Construct the address string using the parsed port
	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Starting server on %s...", addr) // Use Printf for formatting
	log.Fatal(http.ListenAndServe(addr, nil))    // Use the constructed address
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
