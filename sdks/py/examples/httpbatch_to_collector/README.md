# HTTP batch to collector

Run the collector:

```bash
cd collector
go run ./cmd/loxa-collector run -c configs/loxa.local.yaml
```

If auth is enabled on the collector, set:

```bash
export LOXA_API_KEY="lx_local_dev_mydevtoken"
```

Run the Python example:

```bash
cd sdks/py/examples/httpbatch_to_collector
python main.py
```

Use this example to validate that:

- `loxa-py` (Python SDK) emits collector-compatible JSON
- the collector accepts `/events`
- collector-side durability and sink fanout stay outside the SDK
