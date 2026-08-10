# Armbian Hastebin

Armbian Hastebin is a lightweight pastebin server written in Go with ANSI color rendering. It began as a fork of [haste-server](https://github.com/shykes/haste-server), then was migrated from Node.js to Go. Pastes can be stored on the filesystem, Redis, Memcached, MongoDB, PostgreSQL, or S3-compatible object storage.

## Features

- ANSI and syntax-highlighted paste rendering
- Responsive interface with a collapsible mobile toolbar
- Random or human-readable (phonetic) paste keys
- Filesystem, Redis, Memcached, MongoDB, PostgreSQL, and S3 storage backends
- Optional per-paste **delete after** lifetime
- Background expired-paste cleanup for the file backend
- IP-based rate limiting, Prometheus metrics, and a health endpoint
- Request-body, HTTP-header, and server timeout limits

## Quick start

### Docker Compose

```bash
docker compose up --build
```

The application listens on `http://localhost:7777` by default. Paste files are persisted under `./data`.

Static assets are embedded in the Go binary. Rebuild the image after changing the UI:

```bash
docker compose up --build
```

If the browser still serves an old JavaScript or CSS asset, use a hard refresh (`Ctrl+Shift+R`).

### Local development

Go 1.26 or newer is required.

```bash
go run ./cmd --config config.yaml
```

Run the test suite:

```bash
go test ./...
```

Run allocation and throughput benchmarks:

```bash
go test -run '^$' -bench . -benchmem ./handler ./internal/storage ./internal/keygenerator
```

## Configuration

The server reads `config.yaml` from the working directory by default. Use `--config /path/to/config.yaml` to select another file.

Example:

```yaml
host: "0.0.0.0"
port: 7777
key_length: 10
max_length: 4000000 # bytes
key_generator: "phonetic" # phonetic or random
key_space: "abcdefghijklmnopqrstuvwxyz"

storage:
  type: "file"
  file_path: "./data"
  compression: "none" # none, gzip, or zstd

delete_after:
  enable: true

rate_limiting:
  enable: true
  limit: 500
  window: 15 # seconds
  trusted_proxy_count: 0

logging:
  level: "info"
  colorize: true

documents:
  - key: "about"
    path: "./about.md"
```

### Main settings

| Key | Description |
|---|---|
| `host`, `port` | HTTP bind address and port. |
| `key_length` | Length of generated paste keys. |
| `key_generator` | `phonetic` or `random`. |
| `key_space` | Character set used only by the `random` generator. |
| `max_length` | Maximum accepted paste-body size in bytes. |
| `expiration` | Default backend TTL in seconds. `0` disables it. |
| `documents` | Permanent static documents loaded at startup. |

### Storage settings

Supported `storage.type` values: `file`, `redis`, `memcached`, `mongodb`, `postgres`, and `s3`.

| Field | Used by |
|---|---|
| `file_path` | `file` |
| `compression` | `file`; `none`, `gzip`, or `zstd` |
| `host`, `port` | Redis, Memcached, MongoDB, PostgreSQL, and S3 endpoint |
| `username`, `password` | Redis, MongoDB, PostgreSQL, and S3 |
| `database` | MongoDB and PostgreSQL |
| `bucket`, `aws_region` | S3 |

The PostgreSQL table and required migrations are created during startup.

### Rate limiting and proxies

`trusted_proxy_count: 0` is the safe default: the rate-limit key uses the direct TCP peer IP and ignores client-provided `X-Forwarded-For` headers.

If the service is behind a known number of trusted reverse proxies, for example one Nginx or Traefik proxy:

```yaml
rate_limiting:
  trusted_proxy_count: 1
```

Only configure this when those proxies reliably overwrite and forward client-IP headers.

### Environment variables

Most YAML values can be overridden with environment variables:

```text
HOST, PORT, KEY_LENGTH, MAX_LENGTH, STATIC_MAX_AGE
KEY_GENERATOR
STORAGE_TYPE, STORAGE_HOST, STORAGE_PORT, STORAGE_USERNAME,
STORAGE_PASSWORD, STORAGE_DATABASE, STORAGE_BUCKET,
STORAGE_AWS_REGION, STORAGE_FILE_PATH, STORAGE_COMPRESSION
LOGGING_LEVEL, LOGGING_COLORIZE
RATE_LIMITING_ENABLE, RATE_LIMITING_LIMIT, RATE_LIMITING_WINDOW,
RATE_LIMITING_TRUSTED_PROXY_COUNT
DELETE_AFTER_ENABLE
```

Additional static documents can be added as `DOCUMENTS_<key>=<path>`.

## API

| Operation | Endpoint | Response |
|---|---|---|
| Create a paste | `POST /documents` | `{ "key": "...", "expires_at": "..." }` |
| Haste-client-compatible upload | `PUT /log` or `POST /log` | Direct URL |
| Read a paste | `GET /documents/{key}` | `{ "key": "...", "data": "..." }` |
| Read raw content | `GET /raw/{key}` | `text/plain` |
| Health check | `GET /health` | `200 OK` |
| Metrics | `GET /metrics` | Prometheus metrics |

Paste creation accepts a plain-text request body or a `multipart/form-data` request with a `data` field.

### Delete after

Enable the feature with `delete_after.enable: true`. API clients can provide an `X-Delete-After` header:

```bash
curl -X POST http://localhost:7777/documents \
  -H 'Content-Type: text/plain' \
  -H 'X-Delete-After: 1h' \
  --data 'temporary paste'
```

The value uses Go duration syntax: `10m`, `1h`, `24h`, or `168h`. The maximum is 30 days. `never`, or omitting the header, creates a permanent paste.

- Redis and Memcached use native TTL support.
- MongoDB uses a TTL index.
- PostgreSQL stores a fixed expiry timestamp.
- The file backend stores `.expires` metadata and cleans expired pastes at startup and every 10 minutes.
- S3 stores expiry metadata and deletes an expired object when it is read. Configure an S3 bucket lifecycle policy to remove expired objects that are never read.

When `delete_after.enable: false`, the UI selector is hidden and API requests with `X-Delete-After` return `403`.