package swredis

import "github.com/go-redis/redis/v8"

var redisAllowScript = redis.NewScript(`
local key = KEYS[1] -- Identifier for the rate limit (e.g., user ID, IP address)
local now = tonumber(ARGV[1]) -- Current time in milliseconds
local windowSizeMillis = tonumber(ARGV[2]) -- Window size in milliseconds
local limit = tonumber(ARGV[3]) -- The maximum allowed requests

-- Field names in the Redis Hash
local FIELD_PREV_COUNT = 'pc'
local FIELD_CUR_COUNT = 'cc'
local FIELD_CUR_WINDOW_START = 'cws'

-- Get the current state of the counter for this identifier
-- Returns a table: {previousWindowCount, currentWindowCount, currentWindowStart}
local currentCounter = redis.call('HMGET', key, FIELD_PREV_COUNT, FIELD_CUR_COUNT, FIELD_CUR_WINDOW_START)

local previousWindowCount = tonumber(currentCounter[1]) or 0
local currentWindowCount = tonumber(currentCounter[2]) or 0
local currentWindowStart = tonumber(currentCounter[3]) or 0

-- If the counter is new or very old, initialize it
if currentWindowStart == 0 or now - currentWindowStart >= windowSizeMillis * 2 then
    -- Reset both counts if we're well outside the current window
    previousWindowCount = 0
    currentWindowCount = 1
    currentWindowStart = now - (now % windowSizeMillis) -- Truncate to the start of the current window
else
    local timeSinceWindowStart = now - currentWindowStart

    if timeSinceWindowStart >= windowSizeMillis then
        -- We've entered a new window
        -- If currentWindowStart is sufficiently old, we just reset
        if timeSinceWindowStart < 2 * windowSizeMillis then
            previousWindowCount = currentWindowCount
            currentWindowCount = 0
        else
            -- If we skipped more than one window, reset both
            currentWindowCount = 0
            previousWindowCount = 0
        end
        currentWindowStart = now - (now % windowSizeMillis) -- Truncate to the start of the current window
    end
    currentWindowCount = currentWindowCount + 1 -- Increment only after potential window shift
end

-- Calculate total requests using weighted average
local weightCurrentWindow = 0
local weightPreviousWindow = 0

-- Avoid division by zero if windowSizeMillis is 0 (shouldn't happen in practical use)
if windowSizeMillis > 0 then
    local timeSinceWindowStartForWeight = now - currentWindowStart
    if timeSinceWindowStartForWeight < 0 then timeSinceWindowStartForWeight = 0 end -- Should not be negative

    weightCurrentWindow = timeSinceWindowStartForWeight / windowSizeMillis
    if weightCurrentWindow > 1 then weightCurrentWindow = 1 end -- Cap at 1

    weightPreviousWindow = 1 - weightCurrentWindow
end

local totalRequests = (weightCurrentWindow * currentWindowCount) + (weightPreviousWindow * previousWindowCount)

-- Check if limit is exceeded
if totalRequests < limit then
    -- Update the counter in Redis
    redis.call('HMSET', key,
               FIELD_PREV_COUNT, previousWindowCount,
               FIELD_CUR_COUNT, currentWindowCount,
               FIELD_CUR_WINDOW_START, currentWindowStart)
    -- Set expiry on the key to clean up old identifiers.
    -- Set expiry to at least 2 * windowSizeMillis to ensure both current and previous window data is available.
    -- Add some buffer, e.g., an extra window size.
    redis.call('PEXPIRE', key, windowSizeMillis * 3) -- e.g., 3 times the window size for safety
    return 1 -- Allowed
else
    -- Do not update counts if denied
    return 0 -- Denied
end
`)
