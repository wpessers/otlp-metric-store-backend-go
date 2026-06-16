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

## Design

I kept the separate tables per metric type, 1 for gauge and 1 for sum since the histogram, exponential histogram, and summary types were out of scope. The scaffolded tables for those have been kept in the schemas.

The write path now uses 3 tables:
- `otel_metrics_series`: stores resource attributes, scope metadata, metric metadata, attributes, and Sum-specific metadata (aggregation temporality and monotonicity)
- `otel_metrics_gauge_points`: stores Gauge datapoints
- `otel_metrics_sum_points`: stores Sum datapoints

The Gauge and Sum point tables contain only:

- `SeriesID`
- `StartTimeUnix`
- `TimeUnix`
- `Value`
- `Flags`

Metrics metadata is stored once in the `otel_metrics_series` lookup table, and datapoints in the gauge and sum tables reference those rows through a deterministic `SeriesID`.
`SeriesID` is calculated by hashing the identifying metadata of the metrics.

## Limitations

- Currently only focussing on gauge and sum type metrics.
- Possibly missing OTLP edge cases regarding validation, currently only validating that datapoints have `TimeUnixNano != 0`
- `SeriesID` is computed as a 64-bit hash of a canonical representation of the metric series identity. Maybe collision risk is too high, I've opted for this datatype since the assignment explicitly states low cardinality. A production implementation could reduce the risk by using a wider identifier, such as a 128-bit hash. Or some collision detection mechanism.
- The current implementation for calculating the `SeriesID` relies on `json.Marshal` for canonicalization of the series identity before hashing. This is deterministic for the current `map[string]string` fields, but it is not an explicitly versioned canonical format and allocates intermediate JSON bytes. For prod I would probably use a dedicated low-allocation canonical encoder.
- Series rows are deduplicated within a single OTLP export request, for prod I would probably add a cache (with max size and TTL) to deduplicate across requests.
- Rows are inserted into ClickHouse in batches per OTLP export request, batching across requests would be a possible improvement. Alternatively ClickHouse async inserts could be used.
- Production deployment readiness is incomplete: this repository is focused on local development and does not include a Dockerfile, Kubernetes manifests, readiness/liveness probes, or Kubernetes lifecycle configuration. Before running on Kubernetes, the OTLP export request deadline, ClickHouse insert/query timeout, gRPC graceful shutdown timeout, OpenTelemetry and ClickHouse shutdown contexts, optional `preStop` hook, and Kubernetes `terminationGracePeriodSeconds` should be aligned properly so in-flight exports can either complete during shutdown or fail quickly enough for the client to retry.
- No advanced ClickHouse client config: pool sizing, TLS(!) ,...
