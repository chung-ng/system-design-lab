package limiter

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestMemoryTokenBucket_Basic(t *testing.T) {
	cfg := Config{
		Capacity:   5,
		RefillRate: 5.0, // 5 tokens per second
	}
	l := NewMemoryTokenBucket(cfg)
	ctx := context.Background()
	key := "test_client_1"

	// 1. Consume all 5 tokens
	for i := 0; i < 5; i++ {
		res, err := l.Allow(ctx, key)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !res.Allowed {
			t.Fatalf("expected request %d to be allowed", i+1)
		}
	}

	// 2. 6th request must be denied (bucket exhausted)
	res, err := l.Allow(ctx, key)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Allowed {
		t.Fatalf("expected 6th request to be denied, but was allowed")
	}
	if res.RetryAfter <= 0 {
		t.Fatalf("expected RetryAfter > 0, got %v", res.RetryAfter)
	}

	// 3. Sleep 300ms -> should replenish at least 1 token
	time.Sleep(300 * time.Millisecond)

	res, err = l.Allow(ctx, key)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Allowed {
		t.Fatalf("expected request after replenishment to be allowed")
	}
}

func TestMemoryTokenBucket_Concurrency(t *testing.T) {
	cfg := Config{
		Capacity:   100,
		RefillRate: 10.0,
	}
	l := NewMemoryTokenBucket(cfg)
	ctx := context.Background()
	key := "concurrent_client"

	var wg sync.WaitGroup
	var allowedCount int64
	var deniedCount int64

	workers := 200
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			res, err := l.Allow(ctx, key)
			if err != nil {
				return
			}
			if res.Allowed {
				atomic.AddInt64(&allowedCount, 1)
			} else {
				atomic.AddInt64(&deniedCount, 1)
			}
		}()
	}

	wg.Wait()

	if allowedCount > 105 {
		t.Errorf("allowedCount exceeds capacity: got %d, expected ~100", allowedCount)
	}
	if allowedCount+deniedCount != int64(workers) {
		t.Errorf("total requests mismatch: got %d, expected %d", allowedCount+deniedCount, workers)
	}
}

func TestRedisTokenBucket_Integration(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		t.Skip("Local Redis not available on 127.0.0.1:6379, skipping Redis integration test.")
	}

	cfg := Config{
		Capacity:   3,
		RefillRate: 2.0,
		TTL:        30 * time.Second,
	}
	lim, err := NewRedisTokenBucket(client, cfg)
	if err != nil {
		t.Fatalf("failed to create redis limiter: %v", err)
	}
	defer lim.Close()

	testKey := "integration_test_tb_user"
	_ = client.Del(ctx, "rate_limit:"+testKey).Err()

	// Consume 3 tokens
	for i := 0; i < 3; i++ {
		res, err := lim.Allow(ctx, testKey)
		if err != nil || !res.Allowed {
			t.Fatalf("request %d failed: err=%v, res=%+v", i+1, err, res)
		}
	}

	// 4th request must be rejected
	res, err := lim.Allow(ctx, testKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Allowed {
		t.Fatalf("4th request should be rejected")
	}
}

func TestRedisGCRALimiter_Integration(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		t.Skip("Local Redis not available on 127.0.0.1:6379, skipping GCRA integration test.")
	}

	cfg := Config{
		Capacity:   3,
		RefillRate: 2.0, // 1 token every 500ms
	}
	gcra, err := NewRedisGCRALimiter(client, cfg)
	if err != nil {
		t.Fatalf("failed to create GCRA limiter: %v", err)
	}
	defer gcra.Close()

	testKey := "integration_test_gcra_user"
	_ = client.Del(ctx, "rate_limit:gcra:"+testKey).Err()

	// Consume 3 burst requests
	for i := 0; i < 3; i++ {
		res, err := gcra.Allow(ctx, testKey)
		if err != nil || !res.Allowed {
			t.Fatalf("GCRA request %d failed: err=%v, res=%+v", i+1, err, res)
		}
	}

	// 4th request must be rejected by GCRA
	res, err := gcra.Allow(ctx, testKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Allowed {
		t.Fatalf("GCRA 4th request should be rejected")
	}
	if res.RetryAfter <= 0 {
		t.Fatalf("expected positive RetryAfter for GCRA, got %v", res.RetryAfter)
	}
}

func BenchmarkRateLimiters_Comparison(b *testing.B) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	hasRedis := client.Ping(ctx).Err() == nil

	cfg := Config{
		Capacity:   10000000,
		RefillRate: 10000000.0,
		TTL:        60 * time.Second,
	}

	// 1. In-Memory Token Bucket Benchmark
	b.Run("Memory_TokenBucket", func(b *testing.B) {
		memLimiter := NewMemoryTokenBucket(cfg)
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_, _ = memLimiter.Allow(context.Background(), "bench_mem_key")
			}
		})
	})

	if !hasRedis {
		b.Skip("Redis not running, skipping Redis benchmarks")
		return
	}

	// 2. Redis Classic Token Bucket Benchmark
	b.Run("Redis_TokenBucket", func(b *testing.B) {
		tbLimiter, err := NewRedisTokenBucket(client, cfg)
		if err != nil {
			b.Fatal(err)
		}
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_, _ = tbLimiter.Allow(context.Background(), "bench_tb_key")
			}
		})
	})

	// 3. Redis GCRA Benchmark
	b.Run("Redis_GCRA", func(b *testing.B) {
		gcraLimiter, err := NewRedisGCRALimiter(client, cfg)
		if err != nil {
			b.Fatal(err)
		}
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_, _ = gcraLimiter.Allow(context.Background(), "bench_gcra_key")
			}
		})
	})
}
