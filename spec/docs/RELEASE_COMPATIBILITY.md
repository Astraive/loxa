# Release And Compatibility Ownership

Release tasks:

- Go module tags.
- Python PyPI publishing.
- Rust crates.io publishing.
- CLI binary releases.
- Docker images.
- GitHub release notes.
- Changelog generation.
- Version compatibility matrix.

Compatibility matrix:

| Package | Compatibility |
|---|---|
| Go SDK 1.0.x (`sdks/go`) | supports ingest API v1 and stable-v1 SDK parity |
| Python SDK 1.0.x (`sdks/py`) | supports ingest API v1 and stable-v1 SDK parity |
| Rust SDK 1.0.x (`sdks/rs`) | supports ingest API v1 and stable-v1 SDK parity |
| JavaScript SDK 1.0.x (`sdks/js`) | supports ingest API v1 and stable-v1 SDK parity |
| Collector 1.0.x (`collector/`) | accepts ingest API v1 |
| CLI 1.0.x (`cli/`) | operates collector/cortex API v1 surfaces |
| Cortex 1.0.x (`cortex/`) | accepts cortex API v1 surfaces |

Current required SDKs are Go, Python, Rust, and JavaScript.

Future SDKs such as Java/Kotlin and .NET are outside the v1.0.0 stable parity
gate until repositories and conformance tests exist for them.
