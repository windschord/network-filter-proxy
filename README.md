# Network Filter Proxy

A forward proxy written in Go that filters HTTPS CONNECT and plain HTTP requests based on source-IP whitelists. Rules are managed at runtime through a local Management REST API.

## Features

- **HTTPS CONNECT tunneling** and plain HTTP forwarding on port 3128
- **Source-IP based whitelist** with default-deny policy (unregistered IPs receive 403)
- **Flexible matching**: exact domain, wildcard (`*.example.com`), IP address, CIDR, per-port rules
- **Management REST API** bound to `127.0.0.1:8080` (localhost only)
- **Health check** endpoint with uptime, active connections, and rule count
- **Structured logging** (JSON/text) via `log/slog`
- **Graceful shutdown** with configurable timeout and CONNECT tunnel cleanup
- **Lightweight Docker image** (< 30 MB, distroless base)

## Quick Start

### Binary

```bash
go build -o filter-proxy ./cmd/filter-proxy
./filter-proxy
```

### Docker

```bash
docker build -t filter-proxy .
docker run -p 3128:3128 filter-proxy
```

> **Note:** The Management API binds to `127.0.0.1` inside the container. To access it from the host, use `--network host` or a sidecar pattern.

### Register a rule and test

```bash
# Allow 10.0.0.5 to access example.com on any port
curl -X PUT http://127.0.0.1:8080/api/v1/rules/10.0.0.5 \
  -H 'Content-Type: application/json' \
  -d '{"entries":[{"host":"example.com"}]}'

# Proxy a request through the filter
curl -x http://127.0.0.1:3128 http://example.com
```

## Configuration

All settings are controlled via environment variables.

| Variable | Default | Description |
|----------|---------|-------------|
| `PROXY_PORT` | `3128` | Proxy listen port |
| `API_PORT` | `8080` | Management API port (always on `127.0.0.1`) |
| `LOG_FORMAT` | `json` | `json` or `text` |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `SHUTDOWN_TIMEOUT` | `30` | Graceful shutdown timeout in seconds |

## API Reference

Base URL: `http://127.0.0.1:8080`

Full OpenAPI spec: [`docs/swagger/swagger.yaml`](docs/swagger/swagger.yaml)

### Health Check

```http
GET /api/v1/health
```

```json
{
  "status": "ok",
  "uptime_seconds": 3600,
  "active_connections": 5,
  "rule_count": 3
}
```

### List All Rules

```http
GET /api/v1/rules
```

### Set Rules for a Source IP

```http
PUT /api/v1/rules/{sourceIP}
```

```json
{
  "entries": [
    { "host": "api.example.com", "port": 443 },
    { "host": "*.github.com" },
    { "host": "10.0.0.0/8" }
  ]
}
```

- `host` (required): domain, wildcard (`*.example.com`), IP, or CIDR
- `port` (optional): specific port to allow; `0` or omitted means any port

### Delete Rules for a Source IP

```http
DELETE /api/v1/rules/{sourceIP}
```

### Delete All Rules

```http
DELETE /api/v1/rules
```

## Architecture

```plaintext
cmd/filter-proxy/       Entry point
internal/
  config/               Environment variable loading
  logger/               slog factory (JSON/text, timestamp field)
  rule/                 In-memory rule store (sync.RWMutex) + matcher
  proxy/                Proxy handler (goproxy wrapper, tunnel tracking)
  api/                  Management REST API handler
```

### Matching Rules

| Pattern | Example | Matches |
|---------|---------|---------|
| Exact domain | `api.example.com` | `api.example.com` only |
| Wildcard | `*.example.com` | `example.com` and one level below (e.g. `api.example.com`) |
| IP address | `140.82.112.3` | Exact IP (IPv4/IPv6 normalized with `net.IP.Equal`) |
| CIDR | `10.0.0.0/8` | Any IP in the range |

### Security

- **Default deny**: requests from unregistered source IPs are rejected with `403 Forbidden`
- **TLS pass-through**: the proxy does not terminate TLS; it tunnels CONNECT requests end-to-end
- **Localhost-only API**: the Management API binds exclusively to `127.0.0.1`
- **Input validation**: host patterns, ports, source IPs, and JSON payloads are strictly validated

## Development

### Prerequisites

- Go 1.26+
- Node.js (for textlint)

### Run tests

```bash
# Unit + integration tests (excluding E2E)
go test -race $(go list ./... | grep -v /e2e/)

# E2E tests only
go test -race ./e2e/...

# Lint
golangci-lint run ./...

# Markdown lint
npm install
npm run textlint
```

### Regenerate OpenAPI spec

```bash
make swagger
```

### CI

The CI pipeline runs four parallel jobs: **test**, **e2e**, **lint**, and **openapi** freshness check, followed by a **build** job that verifies the Docker image is under 30 MB.

### Release

Push a semver tag to trigger the release workflow:

```bash
git tag v1.0.0
git push origin v1.0.0
```

This builds binaries for linux/darwin (amd64/arm64), pushes a Docker image to GHCR, and creates a GitHub Release with checksums.

## License

MIT
