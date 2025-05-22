package fcinmemory_test

import (
	"context"
	"testing"
	"time"

	fcinmemory "learn.ratelimiter/internal/fixedcounter/inmemory"
)

func BenchmarkFixedCounterInMemory_Allow(b *testing.B) {
	ctx := context.Background()
	identifier := "benchUserFCInMem"

	configs := []struct {
		name       string
		limit      int
		window     time.Duration
		identifier string
	}{
		{"Limit10_Window1s", 10, 1 * time.Second, identifier + "_10_1s"},
		{"Limit1000_Window1s", 1000, 1 * time.Second, identifier + "_1000_1s"},
		{"Limit100000_Window1s", 100000, 1 * time.Second, identifier + "_100k_1s"},
		{"Limit1000_Window100ms", 1000, 100 * time.Millisecond, identifier + "_1000_100ms"},
	}

	for _, config := range configs {
		b.Run(config.name, func(b *testing.B) {
			limiter := fcinmemory.NewLimiter("bench_fc_inmemory_"+config.name, config.window, config.limit)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				// Alternate identifiers to prevent hitting the limit too quickly if b.N is very large
				// and the limit is small. For basic overhead, one identifier is fine if limit is high.
				// Using config.identifier ensures different redis keys if this were a Redis bench.
				_, _ = limiter.Allow(ctx, config.identifier) 
			}
		})
	}
}
