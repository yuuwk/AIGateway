[English](./README.md) | [简体中文](./README.zh-CN.md)

# AI Gateway

A lightweight AI proxy gateway built with Go. It forwards requests based on route prefixes, records request and response data, and provides a simple admin page for browsing call logs.

## Features

- Forwards requests to different upstream services by route prefix
- Preserves the remaining path and query parameters after the prefix
- Stores call logs in MySQL
- Auto-creates the `call_logs` table on startup
- Provides an admin page for viewing request history
- Supports filtering, pagination, and auto-refresh for logs
- Persists request/response payloads (truncated when oversized) for auditing agent behavior (e.g. prompt cache usage)
- Supports Docker-based deployment

## Project Structure

```text
AIGateway/
├─ main.go                     # Application entry point
├─ config.yaml                 # Runtime config file, ignored by .gitignore
├─ docker-compose.yml          # Container startup config
├─ Dockerfile                  # Image build file
├─ aigateway.conf              # Example Nginx reverse proxy config
├─ deploy.sh                   # Linux deployment script
└─ internal/
   ├─ config/                  # Config loading and validation
   ├─ middleware/              # Request logging middleware
   ├─ proxy/                   # Reverse proxy logic
   ├─ store/                   # MySQL access and schema initialization
   └─ web/                     # Admin page and log API
```

## Requirements

- Go 1.24.4 or later
- MySQL 5.7+ / 8.0+
- Docker / Docker Compose (optional)

## Configuration

The application reads `config.yaml` from the project root by default, and you can also pass a custom path:

```bash
./aigateway -config config.yaml
```

Recommended config template:

```yaml
server:
  port: 8080

mysql:
  host: 127.0.0.1
  port: 3306
  user: your_user
  password: your_password
  database: aigateway

routes:
  - prefix: /deepseek/anthropic
    baseUrl: https://api.deepseek.com/anthropic
  - prefix: /openai
    baseUrl: https://api.openai.com
```

Field descriptions:

- `server.port`: gateway listen port
- `mysql.*`: MySQL connection settings
- `routes[].prefix`: public route prefix exposed by the gateway
- `routes[].baseUrl`: upstream service base URL

For example, if the config is:

- `prefix: /deepseek/anthropic`
- `baseUrl: https://api.deepseek.com/anthropic`

Then this request:

```text
POST /deepseek/anthropic/v1/messages
```

Will be forwarded to:

```text
https://api.deepseek.com/anthropic/v1/messages
```

## Local Run

1. Prepare a MySQL database
2. Create `config.yaml` in the project root
3. Install dependencies and start the service

```bash
go mod tidy
go run . -config config.yaml
```

If you prefer to build first:

```bash
go build -o aigateway .
./aigateway -config config.yaml
```

On startup, the service will:

- Connect to MySQL
- Create the `call_logs` table automatically
- Register all configured proxy routes
- Start the admin page and log API

## Admin Page and API

- Admin page: `http://localhost:8080/admin`
- Log API: `http://localhost:8080/api/logs`

Supported query parameters for the log API:

- `route`: filter by route prefix
- `page`: page number, starting from 1
- `pageSize`: page size, default 20, maximum 100

Example:

```text
GET /api/logs?route=/deepseek/anthropic&page=1&pageSize=20
```

## Database Table

The application auto-creates a log table with these fields:

- `route`: matched route prefix
- `method`: HTTP method
- `request_url`: request URL
- `request_body`: request body
- `response_status`: response status code
- `response_body`: response body
- `duration_ms`: request duration
- `created_at`: record creation time

To avoid oversized records, request and response bodies are truncated before being written to the database.

## Auditing Agent Cache Behavior

Because every proxied call is persisted with its request body and response body, you can point an AI coding agent (such as Claude Code) at this gateway and later inspect the logs to see whether the agent is **deliberately bypassing prompt cache** or simply using the API normally.

Typical checks:

- **`cache_read_input_tokens` / `cache_creation_input_tokens`** in streaming responses (`message_start` / `message_delta` events): a healthy multi-turn session should show large `cache_read` on follow-up calls and only small incremental `input_tokens` per turn.
- **`cache_control` markers** in the request body: static blocks (system prompt, tools) should use `cache_control: { type: "ephemeral" }`; volatile data (date, git status, billing headers) should stay **outside** cached breakpoints.
- **Repeated full-context sends**: if `input_tokens` stays high every turn while `cache_read` stays near zero after warmup, that suggests cache is not being reused.
- **Timing gaps**: Anthropic-compatible prompt cache TTL is about 5 minutes; gaps longer than that naturally expire the cache, but repeated 270–300s idle intervals can be a sign of cache-unfriendly pacing.
- **Side requests**: parallel calls (title generation, security checks, etc.) are separate API requests and do not by themselves prove intentional cache bypass on the main conversation.

Export logs from the admin page or query MySQL directly, then analyze patterns across a session. Note that bodies are truncated at 65,535 characters, so very long conversations may look identical in `request_body` even when the live request differed—use `usage` fields in `response_body` as the source of truth for token and cache metrics.

## Docker Deployment

The repository already includes `Dockerfile` and `docker-compose.yml`.

### Option 1: Docker Compose

Prepare `config.yaml`, then run:

```bash
docker compose up -d --build
```

Default port mapping:

- Container port: `8080`
- Host port: `${SERVER_PORT:-8080}`

Stop the service:

```bash
docker compose down
```

### Option 2: deploy.sh

For Linux environments:

```bash
chmod +x deploy.sh
./deploy.sh
```

What the script does:

- Attempts to build the Linux binary
- Runs `docker compose up -d --build`
- Checks whether `/admin` becomes reachable

## Nginx Reverse Proxy Example

The included `aigateway.conf` shows how to expose the gateway under `/aigateway/` and forward requests to local port `8080`.

For example:

- External URL: `http://your-host/aigateway/admin`
- Forwarded to: `http://127.0.0.1:8080/admin`
