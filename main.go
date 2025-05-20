// Package main is the entry point for the rate limiter application.
package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"os" // Import os for stderr
	"strings"
	"time" // Import time for zerolog

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"     // Import zerolog
	"github.com/rs/zerolog/log" // Import zerolog's global logger

	ratelimiter "learn.ratelimiter/api"
	"learn.ratelimiter/metrics"
	"learn.ratelimiter/middleware"
	// Import types to use types.Limiter
)

// main is the entry point of the application.
// It parses flags, loads configuration, initializes rate limiters, sets up HTTP routes with middleware,
// and starts the HTTP server.
func main() {
	// Configure zerolog for console output
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})

	// Define flags
	port := flag.Int("p", 8080, "Port to run the HTTP server on")
	configPath := flag.String("config", "config.yaml", "Path to the configuration file")
	logLevelStr := flag.String("log-level", "info", "Logging level (trace, debug, info, warn, error, fatal, panic)") // Add log level flag

	// Parse the command-line flags
	flag.Parse()

	// Set the global log level based on the flag
	logLevel, err := zerolog.ParseLevel(*logLevelStr)
	if err != nil {
		log.Fatal().Err(err).Str("log_level", *logLevelStr).Msg("Invalid log level provided")
	}
	zerolog.SetGlobalLevel(logLevel)

	log.Info().Str("config_path", *configPath).Msg("Starting application initialization")

	// Use the new function to initialize multiple limiters and get the closer
	limiters, limiterConfigs, closer, err := ratelimiter.NewLimitersFromConfigPath(*configPath)
	if err != nil {
		// Use logger.Fatal for fatal errors
		log.Fatal().Err(err).Str("config_path", *configPath).Msg("Application startup failed: Error initializing rate limiters from config")
	}
	// Defer the Close method on the returned closer
	defer closer.Close()

	log.Info().Msg("All rate limiters successfully initialized.")

	// Retrieve specific limiters from the map
	apiRateLimiterKey := "api_rate_limit"
	apiRateLimiter, ok := limiters[apiRateLimiterKey]
	if !ok {
		// Use logger.Fatal for fatal errors
		log.Fatal().Str("limiter_key", apiRateLimiterKey).Msg("Application startup failed: Rate limiter key not found in config")
	}
	apiRateLimiterConfig, ok := limiterConfigs[apiRateLimiterKey]
	if !ok {
		log.Fatal().Str("limiter_key", apiRateLimiterKey).Msg("Application startup failed: Rate limiter config not found")
	}

	userLoginRateLimiterKey := "user_login_rate_limit_distributed"
	userLoginRateLimiter, ok := limiters[userLoginRateLimiterKey]
	if !ok {
		// Use logger.Fatal for fatal errors
		log.Fatal().Str("limiter_key", userLoginRateLimiterKey).Msg("Application startup failed: Rate limiter key not found in config")
	}
	userLoginRateLimiterConfig, ok := limiterConfigs[userLoginRateLimiterKey]
	if !ok {
		log.Fatal().Str("limiter_key", userLoginRateLimiterKey).Msg("Application startup failed: Rate limiter config not found")
	}

	// You can now use different limiters for different routes or logic
	apiMetrics := metrics.NewRateLimitMetrics()
	userLoginMetrics := metrics.NewRateLimitMetrics()

	// Pass the limiter key and algorithm to the middleware constructor
	apiRateLimitMiddleware := middleware.NewRateLimitMiddleware(apiRateLimiter, apiMetrics, apiRateLimiterKey, apiRateLimiterConfig.Algorithm)
	userLoginRateLimitMiddleware := middleware.NewRateLimitMiddleware(userLoginRateLimiter, userLoginMetrics, userLoginRateLimiterKey, userLoginRateLimiterConfig.Algorithm)

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
	}, getClientIP))

	// Expose Prometheus metrics endpoint
	http.Handle("/metrics", promhttp.Handler())

	// Construct the address string using the parsed port
	addr := fmt.Sprintf(":%d", *port)
	log.Info().Str("address", addr).Msg("Starting HTTP server")
	// Use logger.Fatal for fatal errors from ListenAndServe
	log.Fatal().Err(http.ListenAndServe(addr, nil)).Str("address", addr).Msg("HTTP server stopped")
}

// getClientIP extracts the client's IP address from the request.
// It checks X-Forwarded-For, X-Real-IP headers, and finally the request's RemoteAddr.
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
