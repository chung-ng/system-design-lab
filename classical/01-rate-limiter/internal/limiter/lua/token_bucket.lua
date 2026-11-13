-- Atomic Token Bucket Algorithm in Redis Lua
-- KEYS[1]: Rate limit key (e.g., "rate_limit:user:123")
-- ARGV[1]: Capacity (maximum tokens in the bucket)
-- ARGV[2]: Refill rate per second (tokens added per second)
-- ARGV[3]: Current timestamp in milliseconds (now_ms)
-- ARGV[4]: Requested tokens (usually 1)
-- ARGV[5]: Key expiration TTL in seconds

local key = KEYS[1]
local capacity = tonumber(ARGV[1])
local refill_rate = tonumber(ARGV[2])
local now_ms = tonumber(ARGV[3])
local requested = tonumber(ARGV[4])
local ttl_seconds = tonumber(ARGV[5])

-- Fetch current state: tokens and last_refill_ms
local data = redis.call("HMGET", key, "tokens", "last_refill_ms")
local tokens
local last_refill_ms

if not data[1] or not data[2] then
    -- Key does not exist: Initialize bucket at full capacity
    tokens = capacity
    last_refill_ms = now_ms
else
    tokens = tonumber(data[1])
    last_refill_ms = tonumber(data[2])

    -- Calculate newly replenished tokens since last refill
    local elapsed_ms = math.max(0, now_ms - last_refill_ms)
    local refill_per_ms = refill_rate / 1000.0
    local replenished = elapsed_ms * refill_per_ms
    tokens = math.min(capacity, tokens + replenished)
    last_refill_ms = now_ms
end

-- Check if sufficient tokens are available
if tokens >= requested then
    tokens = tokens - requested
    redis.call("HMSET", key, "tokens", tokens, "last_refill_ms", last_refill_ms)
    redis.call("EXPIRE", key, ttl_seconds)
    -- Return: [allowed = 1, remaining_tokens, retry_after_ms = 0]
    return {1, math.floor(tokens), 0}
else
    -- Insufficient tokens: Calculate wait time until enough tokens are replenished
    local missing_tokens = requested - tokens
    local retry_after_ms = math.ceil((missing_tokens / refill_rate) * 1000.0)
    
    redis.call("HMSET", key, "tokens", tokens, "last_refill_ms", last_refill_ms)
    redis.call("EXPIRE", key, ttl_seconds)
    -- Return: [allowed = 0, remaining_tokens, retry_after_ms]
    return {0, math.floor(tokens), retry_after_ms}
end
