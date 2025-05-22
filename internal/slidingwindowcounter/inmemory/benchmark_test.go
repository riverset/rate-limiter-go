package swinmemory_test

import (
	"context"
	"testing"
	"time"

	swinmemory "learn.ratelimiter/internal/slidingwindowcounter/inmemory"
)

func BenchmarkSlidingWindowInMemory_Allow(b *testing.B) {
	ctx := context.Background()
	baseIdentifier := "benchUserSWInMem"

	configs := []struct {
		name       string
		limit      int
		window     time.Duration
		identifier string
	}{
		{"Limit10_Window1s", 10, 1 * time.Second, baseIdentifier + "_10_1s"},
		{"Limit1000_Window1s", 1000, 1 * time.Second, baseIdentifier + "_1000_1s"},
		{"Limit100000_Window1m", 100000, 1 * time.Minute, baseIdentifier + "_100k_1m"}, // Larger window
		{"Limit1000_Window100ms", 1000, 100 * time.Millisecond, baseIdentifier + "_1000_100ms"},
	}

	for _, config := range configs {
		b.Run(config.name, func(b *testing.B) {
			// It's important that the limiter key is unique if these were to run concurrently
			// or if state could leak, but for in-memory and sequential b.Run, it's less critical.
			limiter := swinmemory.NewLimiter("bench_sw_inmemory_"+config.name, config.window, config.limit)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = limiter.Allow(ctx, config.identifier)
			}
		})
	}
}
