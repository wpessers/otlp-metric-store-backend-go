package main

import (
	"flag"
	"log/slog"
	"os"
	"strconv"
	"time"
)

// returns the value of the environment variable named key, or def when unset
func envOrDefault(key, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return def
}

// returns the int value of the environment variable named key, or def when unset
func envIntOrDefault(key string, def int) int {
	v, ok := os.LookupEnv(key)
	if !ok {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		slog.Warn("invalid integer env var, using default",
			slog.String("key", key), slog.String("value", v), slog.Int("default", def))
		return def
	}
	return n
}

// returns the duration value of the environment variable named key, or def when unset
func envDurationOrDefault(key string, def time.Duration) time.Duration {
	v, ok := os.LookupEnv(key)
	if !ok {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		slog.Warn("invalid duration env var, using default",
			slog.String("key", key), slog.String("value", v), slog.Duration("default", def))
		return def
	}
	return d
}

var (
	listenAddr            = flag.String("listenAddr", envOrDefault("LISTEN_ADDR", "localhost:4317"), "The listen address (env LISTEN_ADDR)")
	maxReceiveMessageSize = flag.Int("maxReceiveMessageSize", envIntOrDefault("MAX_RECEIVE_MESSAGE_SIZE", 16777216), "The max message size in bytes the server can receive (env MAX_RECEIVE_MESSAGE_SIZE)")
	shutdownTimeout       = flag.Duration("shutdownTimeout", envDurationOrDefault("SHUTDOWN_TIMEOUT", 10*time.Second), "Max wait for graceful shutdown before forcing stop (env SHUTDOWN_TIMEOUT)")

	clickhouseAddr     = flag.String("clickhouseAddr", envOrDefault("CLICKHOUSE_ADDR", "localhost:9000"), "ClickHouse native address host:port (env CLICKHOUSE_ADDR)")
	clickhouseDatabase = flag.String("clickhouseDatabase", envOrDefault("CLICKHOUSE_DATABASE", "default"), "ClickHouse database (env CLICKHOUSE_DATABASE)")
	clickhouseUsername = flag.String("clickhouseUsername", envOrDefault("CLICKHOUSE_USERNAME", "default"), "ClickHouse username (env CLICKHOUSE_USERNAME)")
	clickhousePassword = flag.String("clickhousePassword", envOrDefault("CLICKHOUSE_PASSWORD", ""), "ClickHouse password (prefer the CLICKHOUSE_PASSWORD env var)")
)
