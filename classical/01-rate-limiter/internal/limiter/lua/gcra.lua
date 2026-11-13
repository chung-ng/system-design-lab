-- GCRA (Generic Cell Rate Algorithm) in Redis Lua
-- KEYS[1]: Rate limit key (e.g., "rate_limit:gcra:user_123")
-- ARGV[1]: Emission interval T in milliseconds (period per token = 1000 / refill_rate)
-- ARGV[2]: Burst tolerance tau in milliseconds (burst allowance = capacity * T)
-- ARGV[3]: Current timestamp in milliseconds (now_ms)
-- ARGV[4]: Cost in tokens (usually 1)

local key = KEYS[1]
local T = tonumber(ARGV[1])
local tau = tonumber(ARGV[2])
local now_ms = tonumber(ARGV[3])
local cost = tonumber(ARGV[4]) or 1

-- Fetch existing Theoretical Arrival Time (TAT)
local tat = tonumber(redis.call("GET", key)) or now_ms

-- If TAT is in the past, reset it to current time
if tat < now_ms then
    tat = now_ms
end

-- Calculate new prospective TAT after consuming 'cost' tokens
local increment = cost * T
local new_tat = tat + increment

-- Check if new_tat exceeds allowed burst tolerance limit
if new_tat - now_ms > tau then
    -- REJECTED: Request arrived too early
    local retry_after_ms = math.ceil(new_tat - now_ms - tau)
    local remaining = math.max(0, math.floor((tau - (tat - now_ms)) / T))
    return {0, remaining, retry_after_ms}
else
    -- ALLOWED: Set new TAT with dynamic TTL
    local ttl_seconds = math.ceil((new_tat - now_ms) / 1000) + 1
    if ttl_seconds < 2 then ttl_seconds = 2 end
    redis.call("SET", key, new_tat, "EX", ttl_seconds)

    local remaining = math.max(0, math.floor((tau - (new_tat - now_ms)) / T))
    return {1, remaining, 0}
end
