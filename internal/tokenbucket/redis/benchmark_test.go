package tbredis_test

import (
	"context"
	"testing"
	// "time" // Not strictly needed if using high capacity/rate

	tbredis "learn.ratelimiter/internal/tokenbucket/redis"
	"learn.ratelimiter/internal/testharness/redistest"
)

func BenchmarkTokenBucketRedis_Allow(b *testing.B) {
	ctx := context.Background()
	client := redistest.SetupRedisClient(b)
	if client == nil {
		b.Skip("Redis client not initialized, skipping benchmark.")
		return
	}
	defer client.Close()

	limiterKey := "bench_tb_redis"
	
	baseIdentifier := "benchUserTBRedis"

	configs := []struct {
		name       string
		rate       int
		capacity   int
		identifier string
	}{
		{"Rate10_Cap10", 10, 10, baseIdentifier + "_r10_c10"},
		{"Rate1000_Cap1000", 1000, 1000, baseIdentifier + "_r1k_c1k"},
		{"Rate10_Cap1000", 10, 1000, baseIdentifier + "_r10_c1k"},
		{"Rate1M_Cap1M", 1000000, 1000000, baseIdentifier + "_r1M_c1M"},
	}

	for _, config := range configs {
		limiterKey := "bench_tb_redis_" + config.name
		// Optional: redistest.CleanupRedisKeys(b, client, limiterKey, "")
		// Optional: defer redistest.CleanupRedisKeys(b, client, limiterKey, "")

		b.Run(config.name, func(b *testing.B) {
			limiter := tbredis.NewLimiter(limiterKey, config.rate, config.capacity, client)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = limiter.Allow(ctx, config.identifier)
			}
		})
	}
}
