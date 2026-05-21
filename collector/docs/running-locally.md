# Running Locally

```bash
go run ./cmd/loxa-collector run -c configs/loxa.local.yaml
go run ./cmd/loxa-worker run -c configs/loxa.queue.kafka.yaml
go run ./cmd/loxa-loadgen -url http://localhost:9090/ingest
```
