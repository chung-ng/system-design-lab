# System Design Lab — Distributed Systems & AI Infrastructure in Go

[![Go Report Card](https://goreportcard.com/badge/github.com/chung-ng/system-design-lab)](https://github.com/chung-ng/system-design-lab)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

> **Production-ready implementations, proof-of-concepts, and performance benchmarks for 23 classic distributed systems and modern AI infrastructure challenges.**
>
> Built to L5+/Senior & Principal Systems Architect standards.
>
> **Core Focus:** Concurrency mechanics, low latency, fault tolerance, cost optimization, and rigorous failure-mode reasoning using **Golang**, **Redis**, **PostgreSQL**, and **Kafka/NATS**.

---

## Architecture Matrix & Prompt Index

```
system-design-lab/
├── README.md                               # Master Index & Architecture Guidelines
│
├── classical/                              # 15 Classical Distributed Systems Implementations
│   ├── 01-rate-limiter/                    # Token Bucket & GCRA (Atomic Lua & In-Memory Fallback)
│   ├── 02-idempotent-payments-api/         # Distributed Locking, State Machine & Outbox Pattern
│   ├── 03-distributed-cache/               # Cache-Aside, SingleFlight, LRU & Stampede Prevention
│   ├── 04-notification-system/             # WebSocket/SSE Hub, Worker Pool & Backpressure
│   ├── 05-distributed-job-scheduler/       # Delayed Queue, Leader Election & Worker Lease
│   ├── 06-distributed-id-generator/        # Twitter Snowflake 64-bit ID with Clock Drift Protection
│   ├── 07-chat-messaging-system/           # Real-Time Chat Engine with Redis Pub/Sub Cluster
│   ├── 08-url-shortener/                   # Base62 Encoding, Bloom Filters & Database Sharding
│   ├── 09-log-metrics-pipeline/            # High-Throughput Ingestion, Batching & LSM Mechanics
│   ├── 10-leaderboard-system/              # Real-Time Scoring with Redis Sorted Sets (ZSET)
│   ├── 11-news-feed/                       # Fan-out on Write vs. Read Hybrid Architecture
│   ├── 12-search-autocomplete/             # In-Memory Prefix Trie & Redis Caching
│   ├── 13-multi-tenant-saas-backend/       # Row-Level Security vs. Schema Isolation
│   ├── 14-file-storage-service/            # Resumable Chunk Uploads & S3 Presigned URLs
│   └── 15-oncall-alerting-system/          # Alert Deduplication, Throttling & Escalation Policy
│
└── ai-production/                          # 8 Modern AI Infrastructure Implementations (2026 Era)
    ├── 01-llm-serving-gateway/             # Multi-Provider Routing, Token Streaming (SSE), Fallback
    ├── 02-rag-qa-system/                   # Hybrid Search: BM25 (tsvector) + pgvector (HNSW) & RRF
    ├── 03-semantic-caching/                # Embedding Similarity Cache on Redis (Cosine Distance)
    ├── 04-production-agent-guardrails/     # Input Sanitization, Prompt Injection Defense & Token Bucket
    ├── 05-multi-model-routing-fallback/    # Cost/Latency Routing & Automatic Outage Failover
    ├── 06-ai-observability-tracing/        # OpenTelemetry GenAI Conventions & Cost Analytics
    ├── 07-content-moderation-pipeline/     # Asynchronous Moderation Queue & Human-in-the-Loop
    └── 08-llm-evaluation-pipeline/         # Regression Testing & Automated LLM-as-a-Judge CI/CD
```

---

## Roadmap & Implementation Priorities

### 🏆 Tier 1: Core Architectural Anchors
The non-negotiables for high-throughput backend roles:
1. **[`classical/01-rate-limiter`](classical/01-rate-limiter/)**: Token Bucket & GCRA. Atomic Lua execution in Redis, fail-open vs. fail-closed trade-offs under network partitions.
2. **[`classical/02-idempotent-payments-api`](classical/02-idempotent-payments-api/)**: Zero double-charging guarantee. Idempotency keys, state machines, distributed locking, and the Transactional Outbox pattern.
3. **[`classical/03-distributed-cache`](classical/03-distributed-cache/)**: Eradication of Cache Stampede (Thundering Herd) under 10k concurrent cache misses via Go's `singleflight.Group`.
4. **[`ai-production/01-llm-serving-gateway`](ai-production/01-llm-serving-gateway/)**: High-throughput AI proxy in Go. SSE chunk streaming without buffering, multi-provider failover, and Time-to-First-Token (TTFT) optimization.
5. **[`ai-production/02-rag-qa-system`](ai-production/02-rag-qa-system/)**: Production hybrid retrieval combining lexical BM25 (`tsvector`) with dense vector embeddings (`pgvector` with HNSW index) via Reciprocal Rank Fusion (RRF).

### 🥈 Tier 2: Real-Time & High-Concurrency Systems
* **[`classical/04-notification-system`](classical/04-notification-system/)**: WebSocket connection pools, push notification workers, client drop-handling.
* **[`classical/05-distributed-job-scheduler`](classical/05-distributed-job-scheduler/)**: Two-phase delay queue, worker heartbeat leases, leader election.
* **[`classical/06-distributed-id-generator`](classical/06-distributed-id-generator/)**: High-precision 64-bit Snowflake generator handling NTP clock drift.
* **[`ai-production/03-semantic-caching`](ai-production/03-semantic-caching/)**: Caching semantic queries to eliminate redundant LLM API spend by 40–80%.

---

## Standard Project Skeleton

Every implementation is self-contained, reproducible, and verifiable via Docker and load test scripts:

```
module-name/
├── README.md               # Architecture design, Mermaid diagrams, trade-off matrix & failure modes
├── docker-compose.yml      # Local dependencies (Postgres, Redis, Kafka, Mock APIs)
├── Makefile                # make run, make test, make bench
├── go.mod                  # Self-contained Go module
├── cmd/
│   └── server/
│       └── main.go         # Application bootstrap & graceful shutdown
├── internal/
│   ├── domain/             # Business entities & interfaces
│   ├── service/            # Application logic
│   └── storage/            # Redis / Postgres / InMemory implementations
└── scripts/
    └── load_test.sh        # Benchmarking load scripts (hey, k6, or vegeta)
```

---

## Architectural Principles

Every implementation answers three critical production questions:
1. **Fault Isolation (SPOF):** If Redis or downstream dependencies fail, does the system fail-open or fail-closed? How is cascading failure prevented?
2. **Consistency vs. Latency:** In distributed write scenarios, does the service favor strong consistency (CP) or high availability (AP)?
3. **Resource Efficiency:** Measuring heap allocations, goroutine lifecycle, and p99 latency to minimize compute and cloud footprint.

---

## License

MIT © [chung-ng](https://github.com/chung-ng)
