package redis

import "github.com/go-redis/redis/v8"

// redisAllowScript is the Lua script for the Fixed Window Counter algorithm.
// It increments the counter for the current window and checks if the limit is exceeded.
//
// KEYS[1]: The Redis key for the counter (e.g., "rate_limit:api:user123")
// ARGV[1]: Current timestamp in milliseconds
// ARGV[2]: Window duration in milliseconds
// ARGV[3]: Limit
// ARGV[4]: Expiry time for the key in seconds (should be >= window duration)
//
// Returns 1 if the request is allowed, 0 if denied.
var redisAllowScript = redis.NewScript(`
	local key = KEYS[1]
	local now_ms = tonumber(ARGV[1])
	local window_ms = tonumber(ARGV[2])
	local limit = tonumber(ARGV[3])
	local expiry_sec = tonumber(ARGV[4])

	-- Calculate the start time of the current window
	local window_start_ms = math.floor(now_ms / window_ms) * window_ms

	-- Use the window start time as the field in the hash
	local field = tostring(window_start_ms)

	-- Increment the counter for the current window
	local count = redis.call('HINCRBY', key, field, 1)

	-- Set expiry on the key (the hash) if it's the first increment in this window
	-- We set it to expiry_sec, which should be at least the window duration
	-- This ensures the key eventually expires after the window passes.
	if count == 1 then
		redis.call('EXPIRE', key, expiry_sec)
	end

	-- Check if the count exceeds the limit
	if count <= limit then
		return 1 -- Allowed
	else
		return 0 -- Denied
	end
`)
