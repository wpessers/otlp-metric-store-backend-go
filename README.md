# OTLP Metric Storage (Go)

## Prerequisites

- Go 1.26+
- Docker

## Building

```shell
make build
```

## Running locally

The server connects to ClickHouse on startup, so start ClickHouse first:

```shell
# 1. Start a local ClickHouse instance (native protocol on :9000, HTTP on :8123)
docker compose up -d clickhouse     # or: make compose-up

# 2. Run the server
CLICKHOUSE_PASSWORD=test go run ./...       # or: make run (with the env var set)
```

The clickhouse ports can be overridden using env vars `CLICKHOUSE_HOST_PORT` and `CLICKHOUSE_HTTP_PORT`. It might be helpful, for example if port 9000 is already in use for the clickhouse host port.
If you override the host port, do be aware you need to change the clickhouse address the application connects to as well to be aligned with that. You can use the `CLICKHOUSE_ADDR` env var or similar flag for that as shown below in [Configuration](#configuration). Example:

```shell
# 1. Start a local ClickHouse instance (native protocol on :9000, HTTP on :8123)
CLICKHOUSE_HOST_PORT=9001 docker compose up -d clickhouse     # or: make compose-up

# 2. Run the server
CLICKHOUSE_ADDR=localhost:9001 CLICKHOUSE_PASSWORD=test go run ./...       # or: make run (with the env var set)
```

The server listens for OTLP/gRPC on `localhost:4317` by default.

## Configuration

Every setting can be provided as an environment variable or overridden with the matching
command-line flag. The precedence is **flag > env var > built-in default**.

| Flag | Env var | Default | Description |
|------|---------|---------|-------------|
| `--listenAddr` | `LISTEN_ADDR` | `localhost:4317` | gRPC listen address |
| `--maxReceiveMessageSize` | `MAX_RECEIVE_MESSAGE_SIZE` | `16777216` | Max gRPC receive message size (bytes) |
| `--shutdownTimeout` | `SHUTDOWN_TIMEOUT` | `10s` | Max wait for graceful shutdown before forcing stop (e.g. `15s`, `1m`) |
| `--clickhouseAddr` | `CLICKHOUSE_ADDR` | `localhost:9000` | ClickHouse native address `host:port` |
| `--clickhouseDatabase` | `CLICKHOUSE_DATABASE` | `default` | ClickHouse database |
| `--clickhouseUsername` | `CLICKHOUSE_USERNAME` | `default` | ClickHouse username |
| `--clickhousePassword` | `CLICKHOUSE_PASSWORD` | _(empty)_ | ClickHouse password |
