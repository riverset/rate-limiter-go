package swcmemcache_test

import (
	"context"
	"testing"
	"time"

	swcmemcache "learn.ratelimiter/internal/slidingwindowcounter/memcache"
	"learn.ratelimiter/internal/testharness/memcachetest"
)

func BenchmarkSlidingWindowMemcache_Allow(b *testing.B) {
	ctx := context.Background()
	mcClient := memcachetest.SetupMemcachedClient(b)
	if mcClient == nil {
		b.Skip("Memcached client not initialized, skipping benchmark.")
		return
	}
	adapter := memcachetest.NewMemcacheClientAdapter(mcClient)

	limiterKey := "bench_sw_memcache"
	
	baseIdentifier := "benchUserSWMem"

	configs := []struct {
		name       string
		limit      int
		window     time.Duration
		identifier string
	}{
		{"Limit10_Window1s", 10, 1 * time.Second, baseIdentifier + "_10_1s"},
		{"Limit1000_Window1s", 1000, 1 * time.Second, baseIdentifier + "_1000_1s"},
		{"Limit100000_Window1m", 100000, 1 * time.Minute, baseIdentifier + "_100k_1m"},
		{"Limit1000_Window100ms", 1000, 100 * time.Millisecond, baseIdentifier + "_1000_100ms"},
	}

	for _, config := range configs {
		limiterKey := "bench_sw_memcache_" + config.name
		// Optional: memcachetest.CleanupMemcachedKeys(b, mcClient, []string{fmt.Sprintf("%s:%s", limiterKey, config.identifier)})
		// Optional: defer memcachetest.CleanupMemcachedKeys(b, mcClient, []string{fmt.Sprintf("%s:%s", limiterKey, config.identifier)})

		b.Run(config.name, func(b *testing.B) {
			limiter := swcmemcache.NewLimiter(adapter, limiterKey, config.window, config.limit)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = limiter.Allow(ctx, config.identifier)
			}
		})
	}
}
