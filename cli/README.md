# LOZA CLI

LOZA CLI is the local operator/developer interface for a collector-first LOZA workspace.

Stable collector-facing commands:

- `loza query`
- `loza tail`
- `loza dlq`
- `loza replay`
- `loza delete`
- `loza doctor`

Local repo-backed commands:

- `loza collector run`
- `loza collector config print`
- `loza collector config validate`
- `loza worker run`
- `loza bench`

Schema commands currently shipped:

- `loza schema validate`
- `loza schema fetch`

The CLI no longer assumes nonexistent collector `/control/*` HTTP endpoints for local `collector` and `worker` actions; those commands execute the local Go binaries from `collector_repo_path`.
