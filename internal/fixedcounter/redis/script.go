package redis

import "github.com/go-redis/redis/v8"

// redisAllowScript is the Lua script for the Fixed Window Counter algorithm.
// KEYS[1]: The Redis key for the counter (e.g., "rate_limit:api:user123")
// ARGV[1]: Current timestamp in milliseconds
// ARGV[2]: Window duration in milliseconds
// ARGV[3]: Limit
// ARGV[4]: Expiry time for the key in seconds (should be >= window duration)
// Returns 1 if the request is allowed, 0 if denied.
var redisAllowScript = redis.NewScript(`
	local key = KEYS[1]
	local now_ms = tonumber(ARGV[1])
	local window_ms = tonumber(ARGV[2])
	local limit = tonumber(ARGV[3])
	local expiry_sec = tonumber(ARGV[4])

	local window_start_ms = math.floor(now_ms / window_ms) * window_ms

	local field = tostring(window_start_ms)

	local count = redis.call('HINCRBY', key, field, 1)

	if count == 1 then
		redis.call('EXPIRE', key, expiry_sec)
	end

	if count <= limit then
		return 1
	else
		return 0
	end
`)
