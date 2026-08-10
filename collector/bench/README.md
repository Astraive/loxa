# Collector Benchmarks

This directory contains benchmarks for the collector's core paths: ingest parsing, processing pipeline, and sink operations.

## Running

```bash
cd collector
go test ./bench/... -bench=. -benchmem -count=3
```

## Benchmark Index

| Benchmark | File | What it Measures |
|---|---|---|
| Ingest parsing | `bench/*_test.go` | JSON, array, NDJSON, gzip, zstd parsing throughput |
| DuckDB operations | `internal/sinks/duckdb/*_bench_test.go` | Insert, batch insert, query latency |
| Processing pipeline | `internal/processing/*_test.go` | Full pipeline throughput, dedup latency |

## Interpreting Results

- `ns/op` -- nanoseconds per operation (lower is better)
- `B/op` -- bytes allocated per operation (lower is better)
- `allocs/op` -- number of allocations per operation (lower is better)

Use `-benchmem` to see allocation stats. Use `-count=5` for statistical significance. Use `-cpuprofile` and `-memprofile` for detailed analysis.

## Load Testing

For sustained load testing, use the `loza-loadgen` tool:

```bash
go run ./cmd/loza-loadgen -url http://localhost:9308/ingest -rate 10000 -duration 60s
```

This generates realistic event payloads at the specified rate for the specified duration.
