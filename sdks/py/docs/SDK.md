# SDK

The Python SDK exposes:

- `Config`
- `Logger`
- `Params`
- `HTTPBatchSink`
- lifecycle helpers through `loxa.__init__`

Recommended production shape:

1. build a `Logger(Config.production(...))`
2. attach `HTTPBatchSink("http://collector/v1/events")`
3. create one event per operation
4. let the collector handle storage, retries, fanout, and heavy sink integration
