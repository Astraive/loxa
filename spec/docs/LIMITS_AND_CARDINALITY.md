# Limits And Cardinality

Collector memory limits:

- `max_inflight_requests`
- `max_inflight_events`
- `max_queue_bytes`
- `max_event_bytes`
- `max_attr_count`
- `max_attr_depth`
- `max_string_length`
- memory limiter processor

Event design limits:

- max field count
- max nesting depth
- max key length
- max value size
- reserved field names
- label-promotable fields
- never-label fields

These limits protect Loki, metrics systems, and any sink where high-cardinality
labels can create unbounded storage or index growth.

