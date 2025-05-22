package swredis_test

import (
	"context"
	"testing"
	"time"

	swredis "learn.ratelimiter/internal/slidingwindowcounter/redis"
	"learn.ratelimiter/internal/testharness/redistest"
)

func BenchmarkSlidingWindowRedis_Allow(b *testing.B) {
	ctx := context.Background()
	client := redistest.SetupRedisClient(b)
	if client == nil {
		b.Skip("Redis client not initialized, skipping benchmark.")
		return
	}
	defer client.Close()

	limiterKey := "bench_sw_redis"
	
	baseIdentifier := "benchUserSWRedis"

	configs := []struct {
		name       string
		limit      int64
		window     time.Duration
		identifier string
	}{
		{"Limit10_Window1s", 10, 1 * time.Second, baseIdentifier + "_10_1s"},
		{"Limit1000_Window1s", 1000, 1 * time.Second, baseIdentifier + "_1000_1s"},
		{"Limit100000_Window1m", 100000, 1 * time.Minute, baseIdentifier + "_100k_1m"},
		{"Limit1000_Window100ms", 1000, 100 * time.Millisecond, baseIdentifier + "_1000_100ms"},
	}

	for _, config := range configs {
		limiterKey := "bench_sw_redis_" + config.name
		// Optional: redistest.CleanupRedisKeys(b, client, limiterKey, "")
		// Optional: defer redistest.CleanupRedisKeys(b, client, limiterKey, "")

		b.Run(config.name, func(b *testing.B) {
			limiter := swredis.NewLimiter(limiterKey, config.window, config.limit, client)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = limiter.Allow(ctx, config.identifier)
			}
		})
	}
}
