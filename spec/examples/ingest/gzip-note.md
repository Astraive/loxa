# Gzip Payloads

Collectors may accept gzip-compressed variants of the supported ingest payloads.

The compressed body should be accompanied by:

- `Content-Encoding: gzip`
- a matching content type such as `application/json` or `application/x-ndjson`
