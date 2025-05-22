package tbmemcache_test

import (
	"context"
	"testing"
	// "time"

	tbmemcache "learn.ratelimiter/internal/tokenbucket/memcache"
	"learn.ratelimiter/internal/testharness/memcachetest"
)

func BenchmarkTokenBucketMemcache_Allow(b *testing.B) {
	ctx := context.Background()
	mcClient := memcachetest.SetupMemcachedClient(b)
	if mcClient == nil {
		b.Skip("Memcached client not initialized, skipping benchmark.")
		return
	}
	adapter := memcachetest.NewMemcacheClientAdapter(mcClient)

	limiterKey := "bench_tb_memcache"
	
	baseIdentifier := "benchUserTBMem"

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
		limiterKey := "bench_tb_memcache_" + config.name
		// Optional: memcachetest.CleanupMemcachedKeys(b, mcClient, []string{fmt.Sprintf("token_bucket:%s:%s", limiterKey, config.identifier)})
		// Optional: defer memcachetest.CleanupMemcachedKeys(b, mcClient, []string{fmt.Sprintf("token_bucket:%s:%s", limiterKey, config.identifier)})
		
		b.Run(config.name, func(b *testing.B) {
			limiter := tbmemcache.NewLimiter(limiterKey, config.rate, config.capacity, adapter)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = limiter.Allow(ctx, config.identifier)
			}
		})
	}
}
