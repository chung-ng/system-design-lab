package limiter

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

//go:embed lua/gcra.lua
var gcraLua string

// RedisGCRALimiter implements the Generic Cell Rate Algorithm using Redis.
type RedisGCRALimiter struct {
	client             *redis.Client
	script             *redis.Script
	cfg                Config
	emissionIntervalMs float64
	burstToleranceMs   float64
	memFallback        *MemoryTokenBucket
}

// NewRedisGCRALimiter initializes a Redis-backed GCRA rate limiter.
func NewRedisGCRALimiter(client *redis.Client, cfg Config) (*RedisGCRALimiter, error) {
	if cfg.Capacity <= 0 || cfg.RefillRate <= 0 {
		return nil, ErrInvalidConfig
	}

	// T (emission interval per token in ms)
	emissionIntervalMs := 1000.0 / cfg.RefillRate
	// Tau (burst tolerance limit in ms)
	burstToleranceMs := float64(cfg.Capacity) * emissionIntervalMs

	return &RedisGCRALimiter{
		client:             client,
		script:             redis.NewScript(gcraLua),
		cfg:                cfg,
		emissionIntervalMs: emissionIntervalMs,
		burstToleranceMs:   burstToleranceMs,
		memFallback:        NewMemoryTokenBucket(cfg),
	}, nil
}

// Allow consumes 1 token for the specified key using GCRA.
func (g *RedisGCRALimiter) Allow(ctx context.Context, key string) (*Result, error) {
	return g.AllowN(ctx, key, 1)
}

// AllowN evaluates the rate limit for N tokens via GCRA Lua script.
func (g *RedisGCRALimiter) AllowN(ctx context.Context, key string, n int64) (*Result, error) {
	prefixedKey := fmt.Sprintf("rate_limit:gcra:%s", key)
	nowMs := time.Now().UnixMilli()

	res, err := g.script.Run(
		ctx,
		g.client,
		[]string{prefixedKey},
		int64(g.emissionIntervalMs),
		int64(g.burstToleranceMs),
		nowMs,
		n,
	).Result()

	if err != nil {
		if g.cfg.FailOpen {
			log.Printf("[RateLimiter:GCRA] Redis error: %v. Falling back to in-memory limiter", err)
			return g.memFallback.AllowN(ctx, key, n)
		}
		return nil, fmt.Errorf("%w: %v", ErrExecutionFail, err)
	}

	vals, ok := res.([]interface{})
	if !ok || len(vals) < 3 {
		return nil, fmt.Errorf("%w: unexpected response format from GCRA lua script", ErrExecutionFail)
	}

	allowedInt, _ := vals[0].(int64)
	remaining, _ := vals[1].(int64)
	retryAfterMs, _ := vals[2].(int64)

	isAllowed := allowedInt == 1
	var retryAfter time.Duration
	if !isAllowed && retryAfterMs > 0 {
		retryAfter = time.Duration(retryAfterMs) * time.Millisecond
	}

	resetSec := float64(g.cfg.Capacity-remaining) / g.cfg.RefillRate
	resetAfter := time.Duration(resetSec * float64(time.Second))

	return &Result{
		Allowed:    isAllowed,
		Remaining:  remaining,
		RetryAfter: retryAfter,
		ResetAfter: resetAfter,
	}, nil
}

// Close closes the Redis client connection.
func (g *RedisGCRALimiter) Close() error {
	return g.client.Close()
}
