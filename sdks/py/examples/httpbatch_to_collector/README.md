# HTTP batch to collector

Run the collector:

```bash
cd ../loxa-collector
go run ./cmd/loxa-collector run -c configs/loxa.local.yaml
```

If auth is enabled on the collector, set:

```bash
export LOXA_COLLECTOR_API_KEY=your-key
export LOXA_COLLECTOR_API_KEY_HEADER=X-API-Key
```

Run the Python example:

```bash
cd ../loxa-py/examples/httpbatch_to_collector
python main.py
```

Use this example to validate that:

- `loxa-py` emits collector-compatible JSON
- the collector accepts `/v1/events`
- collector-side durability and sink fanout stay outside the SDK
