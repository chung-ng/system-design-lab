package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/chung-ng/system-design-lab/classical/01-rate-limiter/internal/limiter"
	"github.com/chung-ng/system-design-lab/classical/01-rate-limiter/internal/middleware"
	"github.com/redis/go-redis/v9"
)

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func main() {
	port := getEnv("PORT", "8080")
	redisAddr := getEnv("REDIS_ADDR", "127.0.0.1:6379")
	algorithm := getEnv("RATE_LIMIT_ALGORITHM", "token_bucket")
	capacity, _ := strconv.ParseInt(getEnv("RATE_LIMIT_CAPACITY", "10"), 10, 64)
	refillRate, _ := strconv.ParseFloat(getEnv("RATE_LIMIT_REFILL_RATE", "2.0"), 64)

	cfg := limiter.Config{
		Capacity:   capacity,
		RefillRate: refillRate,
		TTL:        60 * time.Second,
		FailOpen:   true,
	}

	// 1. Initialize Redis Client
	rdb := redis.NewClient(&redis.Options{
		Addr:         redisAddr,
		DialTimeout:  1 * time.Second,
		ReadTimeout:  200 * time.Millisecond,
		WriteTimeout: 200 * time.Millisecond,
		PoolSize:     50,
	})

	var rateLimiter limiter.RateLimiter

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	redisAvailable := rdb.Ping(ctx).Err() == nil

	// Factory pattern to select the rate limiting algorithm
	switch algorithm {
	case "memory":
		log.Println("[INFO] Rate Limiter Algorithm: In-Memory Token Bucket")
		rateLimiter = limiter.NewMemoryTokenBucket(cfg)

	case "gcra":
		if !redisAvailable {
			log.Printf("[WARN] Redis unreachable at %s. Falling back to in-memory limiter", redisAddr)
			rateLimiter = limiter.NewMemoryTokenBucket(cfg)
		} else {
			log.Printf("[INFO] Rate Limiter Algorithm: Redis GCRA (Generic Cell Rate Algorithm) at %s", redisAddr)
			gcraLimiter, err := limiter.NewRedisGCRALimiter(rdb, cfg)
			if err != nil {
				log.Fatalf("Failed to initialize GCRA limiter: %v", err)
			}
			rateLimiter = gcraLimiter
		}

	case "token_bucket":
		fallthrough
	default:
		if !redisAvailable {
			log.Printf("[WARN] Redis unreachable at %s. Falling back to in-memory limiter", redisAddr)
			rateLimiter = limiter.NewMemoryTokenBucket(cfg)
		} else {
			log.Printf("[INFO] Rate Limiter Algorithm: Redis Classic Token Bucket at %s", redisAddr)
			tokenBucketLimiter, err := limiter.NewRedisTokenBucket(rdb, cfg)
			if err != nil {
				log.Fatalf("Failed to initialize Token Bucket limiter: %v", err)
			}
			rateLimiter = tokenBucketLimiter
		}
	}
	defer rateLimiter.Close()

	// 2. Set up HTTP Handlers
	mux := http.NewServeMux()

	// Unprotected health check endpoint
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":    "healthy",
			"algorithm": algorithm,
			"time":      time.Now().UTC().Format(time.RFC3339),
		})
	})

	// Protected resource
	protectedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"message":   "Access granted to protected resource!",
			"algorithm": algorithm,
			"user":      r.Header.Get("X-API-Key"),
			"time":      time.Now().UTC().Format(time.RFC3339),
		})
	})

	// Attach rate limiting middleware
	rateLimitedMux := middleware.RateLimitMiddleware(rateLimiter, nil)(protectedHandler)
	mux.Handle("GET /api/v1/resource", rateLimitedMux)

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	// 3. Graceful Shutdown Routine
	shutdownSignal := make(chan os.Signal, 1)
	signal.Notify(shutdownSignal, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("[INFO] Server listening on http://localhost:%s", port)
		log.Printf("[INFO] Configuration: Capacity=%d, RefillRate=%.1f tokens/sec", capacity, refillRate)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server error: %v", err)
		}
	}()

	<-shutdownSignal
	log.Println("[INFO] Shutting down server gracefully...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("[ERROR] Server forced to shutdown: %v", err)
	}

	log.Println("[INFO] Server terminated cleanly.")
}
