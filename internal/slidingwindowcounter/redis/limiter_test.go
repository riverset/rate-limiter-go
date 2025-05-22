package swredis_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/go-redis/redismock/v8"

	swredis "learn.ratelimiter/internal/slidingwindowcounter/redis" // Alias to avoid conflict
)

// actualSlidingWindowScriptSha is the actual SHA1 of the Lua script used in swredis.Limiter.
const actualSlidingWindowScriptSha = "ce198d0b75f140a201eacb010d888ca6aaf30551" // Actual SHA from error msg
// Note: Similar to fcredis, the actual swredis.Limiter likely uses an unexported script object.
// We mock EvalSha calls assuming this SHA.

// mockTime and mockNowFunc are shared for time injection.
var mockTime = time.Date(2024, time.January, 1, 12, 0, 0, 0, time.UTC)
var mockTimeMillis = mockTime.UnixMilli() // Sliding window uses UnixMilli

func mockNowFunc() func() time.Time {
	return func() time.Time {
		return mockTime
	}
}

func TestNewLimiter_SlidingWindowRedis(t *testing.T) {
	client, _ := redismock.NewClientMock()
	key := "test_sliding_window"
	windowSize := 60 * time.Second
	limit := int64(10)

	// Test with default clock
	limiterDefault := swredis.NewLimiter(key, windowSize, limit, client)
	if limiterDefault == nil {
		t.Fatal("NewLimiter with default clock returned nil")
	}

	// Test with injected clock
	limiterWithClock := swredis.NewLimiter(key, windowSize, limit, client, swredis.WithClock(mockNowFunc()))
	if limiterWithClock == nil {
		t.Fatal("NewLimiter with injected clock returned nil")
	}
	// Re-enable interface check if types.Limiter is intended to be used/checked.
	// For now, ensuring it's a *swredis.limiter (which NewLimiter returns) is implicit.
}

func TestAllow_SlidingWindowRedis(t *testing.T) {
	ctx := context.Background()
	limiterName := "test_allow_sw"
	windowSize := 60 * time.Second
	limit := int64(5)
	identifier := "user789"
	expectedRedisKey := fmt.Sprintf("%s:%s", limiterName, identifier)
	scriptSHA := actualSlidingWindowScriptSha // Use the correct SHA

	// Script arguments: now (dynamic), windowSizeMillis, limit
	mockWindowSizeMillis := windowSize.Milliseconds()
	// `now` is now deterministic via mockTimeMillis

	t.Run("SuccessfulAllowance", func(t *testing.T) {
		db, mock := redismock.NewClientMock()
		limiter := swredis.NewLimiter(limiterName, windowSize, limit, db, swredis.WithClock(mockNowFunc()))

		mock.ExpectEvalSha(scriptSHA, []string{expectedRedisKey}, mockTimeMillis, mockWindowSizeMillis, limit).SetVal(int64(1))

		allowed, err := limiter.Allow(ctx, identifier)
		if err != nil {
			t.Fatalf("Allow failed: %v", err)
		}
		if !allowed {
			t.Fatal("Request unexpectedly denied")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Redis mock expectations not met: %s", err)
		}
	})

	t.Run("SuccessfulDenial", func(t *testing.T) {
		db, mock := redismock.NewClientMock()
		limiter := swredis.NewLimiter(limiterName, windowSize, limit, db, swredis.WithClock(mockNowFunc()))

		mock.ExpectEvalSha(scriptSHA, []string{expectedRedisKey}, mockTimeMillis, mockWindowSizeMillis, limit).SetVal(int64(0))

		allowed, err := limiter.Allow(ctx, identifier)
		if err != nil {
			t.Fatalf("Allow failed: %v", err)
		}
		if allowed {
			t.Fatal("Request unexpectedly allowed")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Redis mock expectations not met: %s", err)
		}
	})

	t.Run("RedisScriptError", func(t *testing.T) {
		db, mock := redismock.NewClientMock()
		limiter := swredis.NewLimiter(limiterName, windowSize, limit, db, swredis.WithClock(mockNowFunc()))
		redisErr := errors.New("redis script error")

		mock.ExpectEvalSha(scriptSHA, []string{expectedRedisKey}, mockTimeMillis, mockWindowSizeMillis, limit).SetErr(redisErr)

		_, err := limiter.Allow(ctx, identifier)
		if err == nil {
			t.Fatal("Expected an error but got nil")
		}
		if !strings.Contains(err.Error(), redisErr.Error()) {
			t.Fatalf("Expected error containing '%s', got '%v'", redisErr.Error(), err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Redis mock expectations not met: %s", err)
		}
	})

	t.Run("UnexpectedResultType", func(t *testing.T) {
		db, mock := redismock.NewClientMock()
		limiter := swredis.NewLimiter(limiterName, windowSize, limit, db, swredis.WithClock(mockNowFunc()))

		mock.ExpectEvalSha(scriptSHA, []string{expectedRedisKey}, mockTimeMillis, mockWindowSizeMillis, limit).SetVal("not an int64")

		_, err := limiter.Allow(ctx, identifier)
		if err == nil {
			t.Fatal("Expected an error for unexpected result type but got nil")
		}
		expectedErrStr := "unexpected result type from Redis script"
		if !strings.Contains(err.Error(), expectedErrStr) {
			t.Fatalf("Expected error containing '%s', got '%s'", expectedErrStr, err.Error())
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Redis mock expectations not met: %s", err)
		}
	})

	t.Run("RedisNilError", func(t *testing.T) {
		db, mock := redismock.NewClientMock()
		limiter := swredis.NewLimiter(limiterName, windowSize, limit, db, swredis.WithClock(mockNowFunc()))

		mock.ExpectEvalSha(scriptSHA, []string{expectedRedisKey}, mockTimeMillis, mockWindowSizeMillis, limit).SetErr(redis.Nil)

		_, err := limiter.Allow(ctx, identifier)
		if err == nil {
			t.Fatal("Expected an error for redis.Nil but got nil")
		}
		if !strings.Contains(err.Error(), redis.Nil.Error()) {
			t.Fatalf("Expected error containing redis.Nil, got %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Redis mock expectations not met: %s", err)
		}
	})
}
