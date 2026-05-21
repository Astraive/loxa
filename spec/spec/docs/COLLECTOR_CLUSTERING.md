# Collector Clustering Model

Supported deployment modes:

- Single-node dev.
- Single-node production.
- Multi-replica stateless collector with external queue.
- Multi-replica collector with local spool plus sticky routing warning.
- Worker pool mode.

Safe and unsafe combinations:

- `local spool + 3 replicas`: each pod has its own spool. This is not global durability.
- `Kafka/Redpanda queue + 3 replicas`: recommended production mode.
- `memory dedupe + 3 replicas`: unsafe for global dedupe.
- `Redis/Postgres dedupe + 3 replicas`: safe for global dedupe.

