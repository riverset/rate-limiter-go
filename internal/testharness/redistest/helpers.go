package redistest

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
	"errors" // Add missing import

	"github.com/go-redis/redis/v8"
)

// GetRedisAddress returns the Redis address, defaulting to "localhost:6379".
// If REDIS_ADDR environment variable is set, it's used.
// If CI environment variable is "true", it defaults to "redis:6379".
func GetRedisAddress() string {
	if addr := os.Getenv("REDIS_ADDR"); addr != "" {
		return addr
	}
	if os.Getenv("CI") == "true" {
		return "redis:6379"
	}
	return "localhost:6379"
}

// SetupRedisClient initializes and returns a Redis client for integration tests.
// It fails the test if connection to Redis cannot be established.
func SetupRedisClient(t *testing.T) *redis.Client {
	t.Helper() // Marks this function as a test helper
	redisAddr := GetRedisAddress()
	t.Logf("Connecting to Redis for integration tests at %s", redisAddr)

	client := redis.NewClient(&redis.Options{
		Addr: redisAddr,
		DB:   0, // Default DB
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second) // Increased timeout
	defer cancel()

	if _, err := client.Ping(ctx).Result(); err != nil {
		// Skip test if Redis is not available, rather than failing hard.
		// This allows unit tests to run even if Redis isn't set up for integration.
		// However, for a dedicated integration test run, one might prefer t.Fatalf.
		// For this task, we'll assume Redis should be available for integration tests.
		t.Fatalf("Failed to connect to Redis at %s: %v. Ensure Redis is running and accessible.", redisAddr, err)
	}
	return client
}

// CleanupRedisKeys scans for keys matching a given pattern prefix and limiter key, then deletes them.
// Example patternPrefix: "fixed_window_counter", "leaky_bucket", etc.
// Example limiterKey: "test_limiter_abc"
// This will scan for "patternPrefix:limiterKey:*"
func CleanupRedisKeys(t *testing.T, client *redis.Client, patternPrefix string, limiterKey string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second) // Increased timeout
	defer cancel()

	// Construct the full scan pattern
	// Ensure there's no double colon if limiterKey is empty, though it shouldn't be for these tests.
	var scanPattern string
	if limiterKey == "" {
		scanPattern = fmt.Sprintf("%s:*", patternPrefix)
	} else {
		scanPattern = fmt.Sprintf("%s:%s:*", patternPrefix, limiterKey)
	}
	
	t.Logf("Cleaning up Redis keys with pattern: %s", scanPattern)

	var allKeysToDelete []string
	var cursor uint64
	count := 0 // To prevent infinite loops in tests if SCAN is misbehaving or too many keys

	for count < 1000 { // Safety break for SCAN loop
		keys, nextCursor, err := client.Scan(ctx, cursor, scanPattern, 50).Result() // Scan 50 keys at a time
		if err != nil {
			// If a specific error like "redis: nil" occurs because no keys match, it's not a failure.
			if errors.Is(err, redis.Nil) || strings.Contains(err.Error(), "redis: nil") { // More robust check for nil error
				t.Logf("No keys found matching pattern '%s' for cleanup.", scanPattern)
				break 
			}
			t.Fatalf("Failed to SCAN for keys with pattern '%s': %v", scanPattern, err)
		}

		if len(keys) > 0 {
			allKeysToDelete = append(allKeysToDelete, keys...)
		}

		if nextCursor == 0 { // Iteration finished
			break
		}
		cursor = nextCursor
		count++
	}
	
	if count >= 1000 {
		t.Logf("Warning: SCAN for cleanup might have been capped at %d iterations for pattern %s", count, scanPattern)
	}

	if len(allKeysToDelete) > 0 {
		deletedCount, err := client.Del(ctx, allKeysToDelete...).Result()
		if err != nil {
			t.Errorf("Failed to DEL keys during cleanup (pattern: %s): %v. Keys: %v", scanPattern, err, allKeysToDelete)
		}
		t.Logf("Cleaned up %d keys matching pattern '%s'. Actual Redis DEL count: %d", len(allKeysToDelete), scanPattern, deletedCount)
	} else {
		t.Logf("No keys to delete for pattern '%s'", scanPattern)
	}
}

// Helper for errors.Is, as it's not directly available in older Go versions if not imported.
// For Go 1.13+ errors.Is is standard. Assuming this environment supports it.
// If not, direct string comparison or custom error types would be needed.
