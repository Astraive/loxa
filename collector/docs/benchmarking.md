# Benchmarking

The collector includes benchmarks for the ingest pipeline, processing stages, and sink operations.

## Running Benchmarks

### All Benchmarks

```bash
cd collector
go test ./bench/... -bench=. -benchmem -count=3
```

### DuckDB Sink Benchmarks

```bash
go test ./internal/sinks/duckdb/... -bench=. -benchmem -count=3
```

### Processing Pipeline Benchmarks

```bash
go test ./internal/processing/... -bench=. -benchmem -count=3
```

### Ingest Parser Benchmarks

```bash
go test ./internal/ingest/... -bench=. -benchmem -count=3
```

## What is Measured

| Benchmark | Module | Description |
|---|---|---|
| `BenchmarkDuckDBInsert` | `internal/sinks/duckdb` | Single-event insert throughput |
| `BenchmarkDuckDBBatchInsert` | `internal/sinks/duckdb` | Batch insert throughput (100, 1000, 10000 events) |
| `BenchmarkDuckDBQuery` | `internal/sinks/duckdb` | Query latency for common patterns |
| `BenchmarkProcess` | `internal/processing` | Full processing pipeline throughput |
| `BenchmarkDedupe` | `internal/processing` | Deduplication check latency |
| `BenchmarkParseJSON` | `internal/ingest` | JSON parsing for single objects |
| `BenchmarkParseArray` | `internal/ingest` | JSON array parsing |
| `BenchmarkParseNDJSON` | `internal/ingest` | NDJSON parsing |
| `BenchmarkParseGzip` | `internal/ingest` | Gzip decompression + parsing |
| `BenchmarkParseZstd` | `internal/ingest` | Zstd decompression + parsing |

## Performance Targets

| Metric | Target | Notes |
|---|---|---|
| Ingest throughput | > 50,000 events/sec | Single-node, direct delivery, DuckDB sink |
| Parse latency (p99) | < 1ms | Single JSON object |
| Processing latency (p99) | < 5ms | Full pipeline: validate, redact, dedup |
| DuckDB insert latency (p99) | < 10ms | Single event |
| DuckDB batch insert (1000) | < 100ms | Micro-batch |
| Memory per event | < 2KB | Steady-state processing |

## Benchmark Environment

Benchmarks are designed to run on a standard development machine. For production-grade benchmarking:

1. Use the `loza-loadgen` tool to generate realistic traffic:

```bash
go run ./cmd/loza-loadgen -url http://localhost:9308/ingest -rate 10000 -duration 60s
```

2. Monitor with Prometheus metrics at `/metrics`.
3. Profile with `pprof` if needed:

```bash
go tool pprof http://localhost:9308/debug/pprof/profile?seconds=30
```

## Benchmark Results

Benchmark results are stored in `bench/` and updated with each release. Run `go test ./bench/... -bench=. -benchmem > bench/results.txt` to capture new results.
