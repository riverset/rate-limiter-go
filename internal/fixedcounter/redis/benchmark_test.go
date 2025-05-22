package fcredis_test

import (
	"context"
	"testing"
	"time"

	fcredis "learn.ratelimiter/internal/fixedcounter/redis"
	"learn.ratelimiter/internal/testharness/redistest"
)

func BenchmarkFixedCounterRedis_Allow(b *testing.B) {
	ctx := context.Background()
	client := redistest.SetupRedisClient(b) // Use testing.B which satisfies testing.TB
	if client == nil {
		b.Skip("Redis client not initialized, skipping benchmark.")
		return
	}
	defer client.Close()

	limiterKey := "bench_fc_redis"
	// It's good practice to clean up keys before and after, though less critical for benchmarks
	// if they use unique keys or if Redis is ephemeral for testing.
	// For this benchmark, we'll assume keys are either unique enough or cleaned externally.
	
	baseIdentifier := "benchUserFCRedis"

	configs := []struct {
		name       string
		limit      int64
		window     time.Duration
		identifier string
	}{
		{"Limit10_Window1s", 10, 1 * time.Second, baseIdentifier + "_10_1s"},
		{"Limit1000_Window1s", 1000, 1 * time.Second, baseIdentifier + "_1000_1s"},
		{"Limit100000_Window1s", 100000, 1 * time.Second, baseIdentifier + "_100k_1s"},
		{"Limit1000_Window100ms", 1000, 100 * time.Millisecond, baseIdentifier + "_1000_100ms"},
	}

	for _, config := range configs {
		limiterKey := "bench_fc_redis_" + config.name
		// Optional: redistest.CleanupRedisKeys(b, client, limiterKey, "")
		// Optional: defer redistest.CleanupRedisKeys(b, client, limiterKey, "")

		b.Run(config.name, func(b *testing.B) {
			limiter := fcredis.NewLimiter(client, limiterKey, config.window, config.limit)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = limiter.Allow(ctx, config.identifier)
			}
		})
	}
}
