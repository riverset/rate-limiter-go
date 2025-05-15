package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"

	"learn.ratelimiter/api"
)

func main() {
	// rateLimiter := api.NewTokenBucketLimiter(1, 10)
	// rateLimiter := api.NewFixedCounterLimiter(20, 20)
	rateLimiter := api.NewSlidingWindowLogLimiter(20, 20)
	http.HandleFunc("/unlimited", func(w http.ResponseWriter, r *http.Request) {

		w.WriteHeader(http.StatusOK) // This sets 200 status code
		fmt.Fprintln(w, "Unlimited! Let's Go!")
	})

	http.HandleFunc("/limited", func(w http.ResponseWriter, r *http.Request) {
		ip := getClientIP(r)

		if rateLimiter.Allow(ip) {
			w.WriteHeader(http.StatusOK) // This sets 200 status code
			fmt.Fprintln(w, "Limited, don't over use me!")
			return
		}
		w.WriteHeader(http.StatusTooManyRequests) // 429 for rate limited requests
		fmt.Fprintln(w, "Rate limit exceeded. Please try again later.")
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
