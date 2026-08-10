# Running Locally

```bash
go run ./cmd/loza-collector run -c configs/loza.local.yaml
go run ./cmd/loza-worker run -c configs/loza.queue.kafka.yaml
go run ./cmd/loza-loadgen -url http://localhost:9308/ingest
```
