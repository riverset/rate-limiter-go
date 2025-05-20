// Package tbredis provides a Redis implementation of the Token Bucket rate limiting algorithm.
package tbredis

import "github.com/go-redis/redis/v8"

// redisAllowScript is the Lua script used by the Redis Token Bucket to atomically check and update the bucket state.
// It takes the bucket key, capacity, rate, current timestamp, and requested tokens as arguments.
var redisAllowScript = redis.NewScript(`
		-- Lua script for Token Bucket algorithm
		-- KEYS[1]: bucket key
		-- ARGV[1]: capacity
		-- ARGV[2]: rate (tokens per second)
		-- ARGV[3]: current timestamp in milliseconds
		-- ARGV[4]: tokens to consume (usually 1)

		local key = KEYS[1]
		local capacity = tonumber(ARGV[1])
		local rate = tonumber(ARGV[2])
		local now = tonumber(ARGV[3])
		local requested = tonumber(ARGV[4])

		local fill_time = capacity / rate
		local ttl = math.ceil(fill_time) * 2 -- Set TTL to twice the fill time as a safety margin

		local bucket_info = redis.call('HMGET', key, 'tokens', 'last_refill_time')
		local tokens = tonumber(bucket_info[1])
		local last_refill_time = tonumber(bucket_info[2])

		if tokens == nil then
			tokens = capacity
			last_refill_time = now
		else
			local time_since_last_refill = now - last_refill_time
			local refill_amount = math.floor(time_since_last_refill * rate / 1000)
			tokens = math.min(capacity, tokens + refill_amount)
			last_refill_time = now
		end

		local allowed = 0
		if tokens >= requested then
			allowed = 1
			tokens = tokens - requested
		end

		redis.call('HMSET', key, 'tokens', tokens, 'last_refill_time', last_refill_time)
		redis.call('EXPIRE', key, ttl)

		return {allowed, tokens}
	`)
