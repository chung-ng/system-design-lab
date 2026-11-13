package limiter

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

//go:embed lua/token_bucket.lua
var tokenBucketLua string

// RedisTokenBucket implements RateLimiter using Redis and atomic Lua scripting.
type RedisTokenBucket struct {
	client     *redis.Client
	script     *redis.Script
	cfg        Config
	memFallback *MemoryTokenBucket
}

// NewRedisTokenBucket initializes a distributed Redis-backed rate limiter.
func NewRedisTokenBucket(client *redis.Client, cfg Config) (*RedisTokenBucket, error) {
	if cfg.Capacity <= 0 || cfg.RefillRate <= 0 {
		return nil, ErrInvalidConfig
	}
	if cfg.TTL <= 0 {
		// Default TTL to twice the time needed to fully refill the bucket
		refillFullSeconds := float64(cfg.Capacity) / cfg.RefillRate
		cfg.TTL = time.Duration(refillFullSeconds*2) * time.Second
		if cfg.TTL < 60*time.Second {
			cfg.TTL = 60 * time.Second
		}
	}

	return &RedisTokenBucket{
		client:      client,
		script:      redis.NewScript(tokenBucketLua),
		cfg:         cfg,
		memFallback: NewMemoryTokenBucket(cfg),
	}, nil
}

// Allow consumes 1 token for the specified key.
func (r *RedisTokenBucket) Allow(ctx context.Context, key string) (*Result, error) {
	return r.AllowN(ctx, key, 1)
}

// AllowN evaluates the rate limit for N tokens atomically via Redis Lua script.
func (r *RedisTokenBucket) AllowN(ctx context.Context, key string, n int64) (*Result, error) {
	prefixedKey := fmt.Sprintf("rate_limit:%s", key)
	nowMs := time.Now().UnixMilli()
	ttlSec := int64(r.cfg.TTL.Seconds())

	// Execute Redis script with timeout protection
	res, err := r.script.Run(
		ctx,
		r.client,
		[]string{prefixedKey},
		r.cfg.Capacity,
		r.cfg.RefillRate,
		nowMs,
		n,
		ttlSec,
	).Result()

	if err != nil {
		// Redis connection failure or timeout
		if r.cfg.FailOpen {
			log.Printf("[RateLimiter] Redis error: %v. Falling back to in-memory limiter (Fail-Open)", err)
			return r.memFallback.AllowN(ctx, key, n)
		}
		return nil, fmt.Errorf("%w: %v", ErrExecutionFail, err)
	}

	// Parse Lua return values: {allowed (0 or 1), remaining, retry_after_ms}
	vals, ok := res.([]interface{})
	if !ok || len(vals) < 3 {
		return nil, fmt.Errorf("%w: unexpected response format from lua script", ErrExecutionFail)
	}

	allowedInt, _ := vals[0].(int64)
	remaining, _ := vals[1].(int64)
	retryAfterMs, _ := vals[2].(int64)

	isAllowed := allowedInt == 1
	var retryAfter time.Duration
	if !isAllowed && retryAfterMs > 0 {
		retryAfter = time.Duration(retryAfterMs) * time.Millisecond
	}

	resetSec := float64(r.cfg.Capacity-remaining) / r.cfg.RefillRate
	resetAfter := time.Duration(resetSec * float64(time.Second))

	return &Result{
		Allowed:    isAllowed,
		Remaining:  remaining,
		RetryAfter: retryAfter,
		ResetAfter: resetAfter,
	}, nil
}

// Close closes the Redis client connection.
func (r *RedisTokenBucket) Close() error {
	return r.client.Close()
}
