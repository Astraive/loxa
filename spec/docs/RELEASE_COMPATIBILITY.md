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
| `loxa-go 1.0.x` | supports ingest API v1 and stable-v1 SDK parity |
| `loxa-py 1.0.x` | supports ingest API v1 and stable-v1 SDK parity |
| `loxa-rs 1.0.x` | supports ingest API v1 and stable-v1 SDK parity |
| `loxa-js 1.0.x` | supports ingest API v1 and stable-v1 SDK parity |
| `loxa-collector 1.0.x` | accepts ingest API v1 |
| `loxa-cli 1.0.x` | operates collector/cortex API v1 surfaces |
| `loxa-cortex 1.0.x` | accepts cortex API v1 surfaces |

Current required SDKs are Go, Python, Rust, and JavaScript.

Future SDKs such as Java/Kotlin and .NET are outside the v1.0.0 stable parity
gate until repositories and conformance tests exist for them.
