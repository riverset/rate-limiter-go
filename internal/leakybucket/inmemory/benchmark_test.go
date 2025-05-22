package lbinmemory_test

import (
	"context"
	"testing"
	// "time" // Not strictly needed for this benchmark if using high capacity/rate

	lbinmemory "learn.ratelimiter/internal/leakybucket/inmemory"
)

func BenchmarkLeakyBucketInMemory_Allow(b *testing.B) {
	ctx := context.Background()
	baseIdentifier := "benchUserLBInMem"

	configs := []struct {
		name       string
		rate       int
		capacity   int
		identifier string
	}{
		{"Rate10_Cap10", 10, 10, baseIdentifier + "_r10_c10"},
		{"Rate1000_Cap1000", 1000, 1000, baseIdentifier + "_r1k_c1k"},
		{"Rate10_Cap1000", 10, 1000, baseIdentifier + "_r10_c1k"},      // Low rate, high capacity
		{"Rate1M_Cap1M", 1000000, 1000000, baseIdentifier + "_r1M_c1M"}, // High throughput
	}

	for _, config := range configs {
		b.Run(config.name, func(b *testing.B) {
			limiter := lbinmemory.NewLimiter("bench_lb_inmemory_"+config.name, config.rate, config.capacity)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = limiter.Allow(ctx, config.identifier)
			}
		})
	}
}
