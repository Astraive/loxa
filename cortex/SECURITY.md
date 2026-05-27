# Cortex Security Controls

## WebSocket Origin Validation (websocket.go)
- **What**: Origin header check uses exact prefix match with port, not open prefix match
- **Why**: `strings.HasPrefix(origin, "http://localhost")` matches `http://localhost.evil.com` -- domain suffix attack
- **Fix**: `origin == "http://localhost" || strings.HasPrefix(origin, "http://localhost:")` (and same for 127.0.0.1)
- **Verify**: `grep -n "HasPrefix.*localhost" internal/api/websocket.go`

## WebSocket Read Limit (websocket.go)
- **What**: `conn.SetReadLimit(1MB)` caps maximum WebSocket frame size
- **Why**: Without a limit, a client can send arbitrarily large frames causing memory exhaustion
- **Verify**: `grep -n "SetReadLimit" internal/api/websocket.go`

## WebSocket Role Enforcement (websocket.go)
- **What**: `ingest_event` and `ingest_batch` actions require writer role via `wsHasWriterRole()`
- **Why**: Defense-in-depth -- HTTP middleware is the primary gate, but WebSocket actions bypass it
- **Verify**: `grep -n "wsHasWriterRole" internal/api/websocket.go`

## GraphQL Query Depth Limit (graphql_server.go)
- **What**: Queries exceeding 10 nesting levels are rejected with `query_too_deep`
- **Why**: Deeply nested queries cause exponential backend work (complexity attack)
- **Verify**: `grep -n "maxGraphQLDepth" internal/api/graphql_server.go`

## GraphQL Body Size Limit (graphql_server.go)
- **What**: `io.LimitReader(r.Body, 1MB)` caps request body size
- **Why**: Without a limit, an attacker can send multi-GB bodies causing OOM
- **Verify**: `grep -n "LimitReader" internal/api/graphql_server.go`

## GraphQL Introspection Comment Bypass (graphql_server.go)
- **What**: Comments are stripped before checking for introspection keywords
- **Why**: `#__schema` bypasses string-match introspection blocks
- **Verify**: `grep -n "graphqlCommentRe" internal/api/graphql_server.go`

## Readyz Health Check (server.go)
- **What**: `/readyz` checks graph, processor, and storage initialization
- **Why**: Returning 200 when dependencies are down causes load balancers to route traffic to broken pods
- **Verify**: `curl -s localhost:8080/readyz | jq .`

## Docker Compose Secret Enforcement (configs/docker-compose.yml)
- **What**: `${POSTGRES_PASSWORD:?Set POSTGRES_PASSWORD environment variable}` -- fails fast if unset
- **Why**: `${VAR:-changeme}` silently uses a weak default in production
- **Verify**: `grep "changeme" configs/docker-compose.yml` (should return nothing)

## K8s Image Pinning (deploy/)
- **What**: All deployment manifests pin images to `:0.2.3` instead of `:latest`
- **Why**: `:latest` is mutable -- a compromised registry push affects all pods on next restart
- **Verify**: `grep -rn ":latest" */deploy/ configs/` (should only match doc examples)
