package lbredis_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time" // Ensure time is imported

	"github.com/go-redis/redis/v8"
	"github.com/go-redis/redismock/v8"

	lbredis "learn.ratelimiter/internal/leakybucket/redis" // Alias to avoid conflict
	"learn.ratelimiter/types"
)

// mockTime and mockNowFunc are shared for time injection.
var mockTime = time.Date(2024, time.January, 1, 12, 0, 0, 0, time.UTC)
// Leakybucket in redis uses UnixNano / Millisecond for its 'now' argument to the script.
var mockTimeMillis = mockTime.UnixNano() / int64(time.Millisecond) 

func mockNowFunc() func() time.Time {
	return func() time.Time {
		return mockTime
	}
}

func TestNewLimiter_LeakyBucketRedis(t *testing.T) {
	client, _ := redismock.NewClientMock()
	key := "test_leaky_bucket"
	rate := 10
	capacity := 5

	// Test with default clock
	limiterDefault := lbredis.NewLimiter(key, rate, capacity, client)
	if limiterDefault == nil {
		t.Fatal("NewLimiter with default clock returned nil")
	}

	// Test with injected clock
	limiterWithClock := lbredis.NewLimiter(key, rate, capacity, client, lbredis.WithClock(mockNowFunc()))
	if limiterWithClock == nil {
		t.Fatal("NewLimiter with injected clock returned nil")
	}
	_, ok := limiterWithClock.(types.Limiter) 
	if !ok {
		t.Fatalf("NewLimiter did not return a types.Limiter that implements the interface")
	}
}

func TestAllow_LeakyBucketRedis(t *testing.T) {
	ctx := context.Background()
	limiterKey := "test_allow_lb"
	rate := 10
	capacity := 5
	identifier := "user456"
	expectedRedisKey := fmt.Sprintf("leaky_bucket:%s:%s", limiterKey, identifier)
	scriptSHA := "deb4ed6e82749bdfaf8ed743e0ff2e95b7e36a66" // Actual SHA from error msg

	t.Run("SuccessfulAllowance", func(t *testing.T) {
		db, mock := redismock.NewClientMock()
		limiter := lbredis.NewLimiter(limiterKey, rate, capacity, db, lbredis.WithClock(mockNowFunc()))
		
		mock.ExpectEvalSha(scriptSHA, []string{expectedRedisKey}, capacity, rate, mockTimeMillis).SetVal(int64(1))
		
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
		limiter := lbredis.NewLimiter(limiterKey, rate, capacity, db, lbredis.WithClock(mockNowFunc()))

		mock.ExpectEvalSha(scriptSHA, []string{expectedRedisKey}, capacity, rate, mockTimeMillis).SetVal(int64(0))

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
		limiter := lbredis.NewLimiter(limiterKey, rate, capacity, db, lbredis.WithClock(mockNowFunc()))
		redisErr := errors.New("redis script error")

		mock.ExpectEvalSha(scriptSHA, []string{expectedRedisKey}, capacity, rate, mockTimeMillis).SetErr(redisErr)


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
		limiter := lbredis.NewLimiter(limiterKey, rate, capacity, db, lbredis.WithClock(mockNowFunc()))
		
		mock.ExpectEvalSha(scriptSHA, []string{expectedRedisKey}, capacity, rate, mockTimeMillis).SetVal("not an int64")

		_, err := limiter.Allow(ctx, identifier)
		if err == nil {
			t.Fatal("Expected an error for unexpected result type but got nil")
		}
		expectedErrStr := "unexpected result type from redis script"
		if !strings.Contains(err.Error(), expectedErrStr) {
			t.Fatalf("Expected error containing '%s', got '%s'", expectedErrStr, err.Error())
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Redis mock expectations not met: %s", err)
		}
	})

	t.Run("RedisNilError", func(t *testing.T) {
		db, mock := redismock.NewClientMock()
		limiter := lbredis.NewLimiter(limiterKey, rate, capacity, db, lbredis.WithClock(mockNowFunc()))

		mock.ExpectEvalSha(scriptSHA, []string{expectedRedisKey}, capacity, rate, mockTimeMillis).SetErr(redis.Nil)

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
