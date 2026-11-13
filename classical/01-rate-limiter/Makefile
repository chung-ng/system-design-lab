.PHONY: up down run test bench load-test

up:
	docker compose up -d

down:
	docker compose down

run:
	go run cmd/server/main.go

test:
	go test -v -race ./...

bench:
	go test -bench=BenchmarkRateLimiters_Comparison -benchmem ./internal/limiter

load-test:
	bash scripts/benchmark.sh
