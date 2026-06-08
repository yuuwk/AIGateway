# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run

```bash
go mod tidy                    # Resolve dependencies
go build -o aigateway.exe .    # Build
./aigateway.exe                # Run with default config.yaml
./aigateway.exe -config prod.yaml  # Run with custom config
```

No tests or linters are configured yet.

## Architecture

This is a reverse-proxy API gateway with audit logging. Three independent subsystems share a MySQL store:

**Proxy layer** (`internal/proxy/`): Wraps `net/http/httputil.ReverseProxy`. Each configured route maps a URL prefix to an upstream base URL. The `Rewrite` callback strips the prefix from the incoming path and reattaches the remainder to the target host. `joinPath` handles the edge case where the target base URL itself contains a path segment.

**Logging middleware** (`internal/middleware/`): Sits between the mux and the proxy. Buffers the request body (restoring it via `io.NopCloser` so the proxy can still read it), then wraps `http.ResponseWriter` with `responseCapturer` to tee the response body into a buffer. After the proxy returns, it fires an async goroutine to INSERT into MySQL — the client is never blocked on DB I/O. Bodies are truncated to 65535 chars.

**Web UI** (`internal/web/`): A single `Handler` that routes by path: `/admin` → serves the HTML template (embedded via `//go:embed`), `/api/logs` → JSON paginated query endpoint. The HTML page uses vanilla JS `fetch()` against `/api/logs` with client-side pagination, route filtering, and expandable row details.

**Store** (`internal/store/`): Thin wrapper over `database/sql` with a MySQL driver. `InitSchema()` runs `CREATE TABLE IF NOT EXISTS` on startup. `QueryLogs` supports optional route filtering and returns total count + page for the pagination UI.

**Config** (`internal/config/`): YAML deserialization into structs. `MySQLConfig.DSN()` builds the connection string. Validation checks that port, host, database, and at least one route are present (defaults MySQL port to 3306 if omitted).

### Startup flow (main.go)

1. Parse `-config` flag → `config.Load()`
2. `store.New(dsn)` → open + ping MySQL pool
3. `store.InitSchema()` → auto-create `call_logs` table
4. Register `/admin`, `/admin/`, `/api/logs` → single web Handler
5. For each configured route: create proxy → wrap in logging middleware → register at `/<prefix>/` and `/<prefix>` on the default mux
6. `http.ListenAndServe` with graceful shutdown on SIGINT/SIGTERM

### URL routing detail

The proxy is registered at both `/prefix` (exact) and `/prefix/` (subtree) so that both `/deepseek/anthropic` and `/deepseek/anthropic/v1/messages` are caught. The `Rewrite` function strips the full prefix — e.g. with prefix `/deepseek/anthropic` and incoming path `/deepseek/anthropic/v1/chat`, the upstream receives `/v1/chat` appended to the target base URL's path.

## Config schema

```yaml
server:
  port: 8080
mysql:
  host: localhost
  port: 3306
  user: root
  password: ""
  database: aigateway
routes:
  - prefix: /deepseek/anthropic   # URL prefix to match on incoming requests
    baseUrl: https://api.deepseek.com/anthropic  # upstream target
```

The database `aigateway` must exist before startup; the `call_logs` table is auto-created.
