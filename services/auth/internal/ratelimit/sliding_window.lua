-- KEYS[1] = ключ (например, "rl:user:42")
-- ARGV[1] = window в миллисекундах
-- ARGV[2] = limit
-- ARGV[3] = member (уникальный, обычно now .. ":" .. random/uuid)

local key = KEYS[1]
local window = tonumber(ARGV[1])
local limit = tonumber(ARGV[2])
local member = ARGV[3]

if window == nil or window <= 0 then
    return redis.error_reply("ratelimit: invalid window argument")
end
if limit == nil or limit <= 0 then
    return redis.error_reply("ratelimit: invalid limit argument")
end
if member == nil or member == "" then
    return redis.error_reply("ratelimit: invalid member argument")
end

local time = redis.call("TIME")

local now = tonumber(time[1]) * 1000 + math.floor(tonumber(time[2]) / 1000)

redis.call("ZREMRANGEBYSCORE", key, 0, now - window)

local count = redis.call("ZCARD", key)
if count >= limit then
    return 0
end

redis.call("ZADD", key, now, member)
redis.call("PEXPIRE", key, window)

return 1