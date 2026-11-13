package limiter

import (
	"context"
	"errors"
	"time"
)

var (
	ErrInvalidConfig = errors.New("rate limiter: invalid configuration")
	ErrExecutionFail = errors.New("rate limiter: execution failed")
)

// Result encapsulates the outcome of a rate-limiting check.
type Result struct {
	Allowed    bool          // Whether the request is permitted
	Remaining  int64         // Remaining tokens in the bucket
	RetryAfter time.Duration // Duration to wait before retrying (if rejected)
	ResetAfter time.Duration // Estimated duration until the bucket is completely full
}

// Config defines the operational parameters for the Token Bucket algorithm.
type Config struct {
	Capacity   int64         // Maximum burst capacity (burst allowance)
	RefillRate float64       // Sustained tokens added per second
	TTL        time.Duration // Key expiry duration in Redis
	FailOpen   bool          // If true, allow requests when Redis or datastore fails
}

// RateLimiter defines the contract for rate limiting implementations.
type RateLimiter interface {
	// Allow checks if 1 token can be consumed for the given key.
	Allow(ctx context.Context, key string) (*Result, error)

	// AllowN checks if n tokens can be consumed for the given key.
	AllowN(ctx context.Context, key string, n int64) (*Result, error)

	// Close cleanly terminates any background routines or connections.
	Close() error
}
