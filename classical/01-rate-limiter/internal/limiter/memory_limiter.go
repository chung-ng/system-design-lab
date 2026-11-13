package limiter

import (
	"context"
	"math"
	"sync"
	"time"
)

type bucket struct {
	tokens     float64
	lastRefill time.Time
}

// MemoryTokenBucket is a thread-safe, in-process Token Bucket implementation.
type MemoryTokenBucket struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	cfg     Config
}

// NewMemoryTokenBucket creates an in-memory token bucket rate limiter.
func NewMemoryTokenBucket(cfg Config) *MemoryTokenBucket {
	if cfg.Capacity <= 0 {
		cfg.Capacity = 10
	}
	if cfg.RefillRate <= 0 {
		cfg.RefillRate = 1.0
	}
	return &MemoryTokenBucket{
		buckets: make(map[string]*bucket),
		cfg:     cfg,
	}
}

func (m *MemoryTokenBucket) Allow(ctx context.Context, key string) (*Result, error) {
	return m.AllowN(ctx, key, 1)
}

func (m *MemoryTokenBucket) AllowN(ctx context.Context, key string, n int64) (*Result, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	b, exists := m.buckets[key]
	if !exists {
		b = &bucket{
			tokens:     float64(m.cfg.Capacity),
			lastRefill: now,
		}
		m.buckets[key] = b
	}

	// Refill tokens
	elapsed := now.Sub(b.lastRefill).Seconds()
	b.tokens = math.Min(float64(m.cfg.Capacity), b.tokens+(elapsed*m.cfg.RefillRate))
	b.lastRefill = now

	needed := float64(n)
	if b.tokens >= needed {
		b.tokens -= needed
		return &Result{
			Allowed:    true,
			Remaining:  int64(b.tokens),
			RetryAfter: 0,
			ResetAfter: time.Duration((float64(m.cfg.Capacity)-b.tokens)/m.cfg.RefillRate) * time.Second,
		}, nil
	}

	missing := needed - b.tokens
	retryAfter := time.Duration((missing/m.cfg.RefillRate)*float64(time.Second))
	return &Result{
		Allowed:    false,
		Remaining:  int64(b.tokens),
		RetryAfter: retryAfter,
		ResetAfter: time.Duration((float64(m.cfg.Capacity)-b.tokens)/m.cfg.RefillRate) * time.Second,
	}, nil
}

func (m *MemoryTokenBucket) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.buckets = nil
	return nil
}
