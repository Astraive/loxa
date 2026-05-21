# LOXA CLI

LOXA CLI is the local operator/developer interface for a collector-first LOXA workspace.

Stable collector-facing commands:

- `loxa query`
- `loxa tail`
- `loxa dlq`
- `loxa replay`
- `loxa delete`
- `loxa doctor`

Local repo-backed commands:

- `loxa collector run`
- `loxa collector config print`
- `loxa collector config validate`
- `loxa worker run`
- `loxa bench`

Schema commands currently shipped:

- `loxa schema validate`
- `loxa schema fetch`

The CLI no longer assumes nonexistent collector `/control/*` HTTP endpoints for local `collector` and `worker` actions; those commands execute the local Go binaries from `collector_repo_path`.
