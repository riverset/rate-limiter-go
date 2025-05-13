package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
)

func main() {
	http.HandleFunc("/unlimited", func(w http.ResponseWriter, r *http.Request) {
		ip := getClientIP(r)
		fmt.Println("IP:", ip)

		fmt.Fprintln(w, "Unlimited! Let's Go!")
	})

	http.HandleFunc("/limited", func(w http.ResponseWriter, r *http.Request) {
		ip := getClientIP(r)
		fmt.Println("IP:", ip)

		fmt.Fprintln(w, "Limited, don't over use me!")
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
