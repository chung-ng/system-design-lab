# Globally Distributed Rate Limiter in Go

[![Go Report Card](https://goreportcard.com/badge/github.com/chung-ng/system-design-lab)](https://github.com/chung-ng/system-design-lab)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

> **Category:** Classical Distributed Systems
> 
> **Objective:** Design and implement a high-throughput, low-latency distributed rate limiter in **Golang & Redis**, supporting both **Classic Token Bucket** and **GCRA (Generic Cell Rate Algorithm)** via atomic Lua scripts with an automatic in-memory Circuit Breaker fallback.

---

## 1. System Requirements

* **Functional Requirements:**
  * Rate limit incoming requests per user ID, API key (`X-API-Key`), or client IP.
  * Support bursty traffic configurations (e.g., sustained 2 req/sec with burst capacity up to 10 tokens).
  * Support dynamic algorithm switching via environment variables (`token_bucket`, `gcra`, or `memory`).
  * Return standard HTTP headers: `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`, and `429 Too Many Requests` (with `Retry-After`) upon violation.
* **Non-Functional Requirements:**
  * **Strict Latency SLA:** Rate-limiting check must complete in **< 1ms** (Redis round-trip) and **< 200ns** (in-memory).
  * **Atomicity & Zero Race Conditions:** All state evaluations and deductions execute atomically in Redis via embedded Lua scripts.
  * **Resilience (Fail-Open):** If Redis crashes or exceeds a 50ms deadline, requests gracefully degrade to local in-memory token buckets, preventing cascading API downtime.

---

## 2. Algorithm Comparison & Trade-offs

| Algorithm | Storage State | Pros | Cons | Ideal Use Case |
| :--- | :--- | :--- | :--- | :--- |
| **Classic Token Bucket** *(Default)* | Redis Hash (`tokens`, `last_refill`) | • Easy to inspect and debug<br>• Naturally handles traffic bursts and dynamic token costs | Uses more memory (2 hash fields per user) | Standard REST APIs, API Gateways, Microservices |
| **GCRA** *(Generic Cell Rate)* | Single Redis Key (`TAT` timestamp) | • **Lowest memory:** Stores only 1 number per user<br>• Smooths traffic evenly | Harder to debug (tracks arrival time, not a token count) | High-scale SaaS (Stripe, GitHub), multi-tenant billing |
| **In-Memory Token Bucket** | Local RAM (`sync.Mutex`) | • **Ultra-fast (~166ns):** Zero network calls | Not distributed (each server instance has its own counter) | Standalone servers, unit testing, Circuit Breaker fallback |

---

## 3. High-Level Architecture

```mermaid
flowchart TD
    Client["Client / Upstream Caller"] -->|HTTP Request with X-API-Key| MW["RateLimitMiddleware"]
    MW --> Extractor["Extract Key ('apikey:...' or 'ip:...')"]
    Extractor --> Guard["50ms Context Timeout Guard"]
    
    Guard --> Factory{"Algorithm Factory\n(RATE_LIMIT_ALGORITHM)"}
    Factory -->|token_bucket| RedisTB["Redis Token Bucket\n(HMGET/HMSET Lua)"]
    Factory -->|gcra| RedisGCRA["Redis GCRA\n(GET/SET TAT Lua)"]
    Factory -->|memory| MemTB["In-Memory Token Bucket\n(Local RAM)"]
    
    RedisTB -->|Executes Atomically| RedisStore[("Central Redis RAM")]
    RedisGCRA -->|Executes Atomically| RedisStore

    RedisStore -.->|Allowed: 200 OK + RateLimit Headers| Backend["Protected API Resource"]
    RedisStore -.->|Denied: 429 Too Many Requests + Retry-After| Client
    
    RedisStore -.->|Timeout / Error (Fail-Open)| MemTB
```

---

## 4. Benchmark Performance & Empirical Results

Benchmarked on **Apple M4 (Darwin arm64)** using parallel Go benchmark routines (`go test -bench=BenchmarkRateLimiters_Comparison -benchmem`):

```
pkg: github.com/chung-ng/system-design-lab/classical/01-rate-limiter/internal/limiter
cpu: Apple M4
```

| Implementation | Operations / sec | Latency (ns/op) | Memory Allocated | Allocs / op |
| :--- | :--- | :--- | :--- | :--- |
| **`Memory_TokenBucket`** | **6,497,863 ops/s** | **166.4 ns/op** | 32 B/op | 1 allocs/op |
| **`Redis_GCRA`** | **41,790 ops/s** | **29,285 ns/op** (0.029 ms) | 600 B/op | 18 allocs/op |
| **`Redis_TokenBucket`** | **40,194 ops/s** | **30,913 ns/op** (0.030 ms) | 632 B/op | 19 allocs/op |

### Key Benchmark Insights:
1. **In-Memory Throughput:** Can process **6.5 Million checks/sec** with sub-microsecond latency, making the fallback engine virtually free of overhead.
2. **GCRA vs. Token Bucket in Redis:** GCRA achieves slightly higher throughput (~41.8k ops/s vs. ~40.2k ops/s) and uses fewer bytes because it operates on a single scalar key (`GET`/`SET`) instead of a Redis Hash (`HMGET`/`HMSET`).

---

## 5. How to Run & Test

### Step 1: Start Redis
```bash
make up
# Or: docker compose up -d
```

### Step 2: Run the Service
Select the desired algorithm using `RATE_LIMIT_ALGORITHM`:

```bash
# Option A: Run with Classic Token Bucket (Default)
RATE_LIMIT_ALGORITHM=token_bucket go run cmd/server/main.go

# Option B: Run with GCRA
RATE_LIMIT_ALGORITHM=gcra go run cmd/server/main.go

# Option C: Run purely in-memory (No Redis dependency)
RATE_LIMIT_ALGORITHM=memory go run cmd/server/main.go
```

### Step 3: Run the Verification Script
In a second terminal, execute the automated 15-request burst test:
```bash
make load-test
# Or: bash scripts/benchmark.sh
```

### Step 4: Run Race Detector & Benchmark Suite
```bash
# Run unit & concurrency tests with data race detection
make test
# (go test -v -race ./...)

# Run side-by-side benchmark comparison
make bench
# (go test -bench=BenchmarkRateLimiters_Comparison -benchmem ./internal/limiter)
```

---

## License

MIT © [chung-ng](https://github.com/chung-ng)
