package memcachetest

import (
	"os"
	// "strings" // Removed unused import
	"testing"

	"github.com/bradfitz/gomemcache/memcache"
	"learn.ratelimiter/internal/memcacheiface"
)

// GetMemcachedAddress returns the Memcached address, defaulting to "localhost:11211".
// If MEMCACHED_ADDR environment variable is set, it's used.
// If CI environment variable is "true", it defaults to "memcached:11211" (common in Docker Compose).
func GetMemcachedAddress() string {
	if addr := os.Getenv("MEMCACHED_ADDR"); addr != "" {
		return addr
	}
	if os.Getenv("CI") == "true" {
		return "memcached:11211"
	}
	return "localhost:11211"
}

// SetupMemcachedClient initializes and returns a real *memcache.Client for integration tests.
// It fails the test if connection to Memcached cannot be established.
func SetupMemcachedClient(t *testing.T) *memcache.Client {
	t.Helper()
	memcachedAddr := GetMemcachedAddress()
	t.Logf("Connecting to Memcached for integration tests at %s", memcachedAddr)

	mc := memcache.New(memcachedAddr)

	// Ping (or a simple Get/Set) to check connectivity.
	// Memcache client doesn't have a direct Ping. Set a dummy key.
	err := mc.Set(&memcache.Item{Key: "ping_test", Value: []byte("1"), Expiration: 10})
	if err != nil {
		t.Fatalf("Failed to connect to Memcached at %s (Set ping_test failed): %v. Ensure Memcached is running and accessible.", memcachedAddr, err)
	}
	// And try to get it back
	_, err = mc.Get("ping_test")
	if err != nil {
		t.Fatalf("Failed to connect to Memcached at %s (Get ping_test failed): %v. Ensure Memcached is running and accessible.", memcachedAddr, err)
	}
	mc.Delete("ping_test") // Clean up ping key
	
	t.Logf("Successfully connected to Memcached at %s", memcachedAddr)
	return mc
}

// CleanupMemcachedKeys deletes the specified keys from Memcached.
// It logs errors but doesn't fail the test, as cleanup is best-effort.
func CleanupMemcachedKeys(t *testing.T, client *memcache.Client, keys []string) {
	t.Helper()
	if len(keys) == 0 {
		return
	}
	t.Logf("Cleaning up Memcached keys: %v", keys)
	var failedDeletes []string
	for _, key := range keys {
		err := client.Delete(key)
		// memcache.ErrCacheMiss means the key didn't exist, which is fine for cleanup.
		if err != nil && err != memcache.ErrCacheMiss {
			t.Logf("Warning: Failed to delete Memcached key '%s': %v", key, err)
			failedDeletes = append(failedDeletes, key)
		}
	}
	if len(failedDeletes) > 0 {
		t.Logf("Finished cleanup. Keys that failed to delete (or were already gone): %v", failedDeletes)
	} else {
		t.Logf("Finished cleanup successfully for keys: %v", keys)
	}
}

// MemcacheClientAdapter wraps a real *memcache.Client to satisfy the memcacheiface.Client.
type MemcacheClientAdapter struct {
	Client *memcache.Client
}

// NewMemcacheClientAdapter creates a new adapter.
func NewMemcacheClientAdapter(client *memcache.Client) *MemcacheClientAdapter {
	return &MemcacheClientAdapter{Client: client}
}

func (a *MemcacheClientAdapter) Get(key string) (*memcache.Item, error) {
	return a.Client.Get(key)
}

func (a *MemcacheClientAdapter) Set(item *memcache.Item) error {
	return a.Client.Set(item)
}

func (a *MemcacheClientAdapter) Add(item *memcache.Item) error {
	return a.Client.Add(item)
}

func (a *MemcacheClientAdapter) Increment(key string, delta uint64) (newValue uint64, err error) {
	return a.Client.Increment(key, delta)
}

// Ensure MemcacheClientAdapter implements memcacheiface.Client
var _ memcacheiface.Client = &MemcacheClientAdapter{}
