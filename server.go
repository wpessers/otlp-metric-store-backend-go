package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const name = "otlp-metrics-store-backend"

var (
	meter                  = otel.Meter(name)
	logger                 = otelslog.NewLogger(name)
	metricsReceivedCounter metric.Int64Counter
)

func init() {
	var err error
	metricsReceivedCounter, err = meter.Int64Counter("com.dash0.homeexercise.metrics.received",
		metric.WithDescription("The number of metrics received by otlp-metrics-store-backend"),
		metric.WithUnit("{metric}"))
	if err != nil {
		panic(err)
	}
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

func run() (err error) {
	slog.SetDefault(logger)
	logger.Info("Starting application")

	// Set up OpenTelemetry.
	otelShutdown, err := setupOTelSDK(context.Background())
	if err != nil {
		return
	}

	// Handle shutdown properly so nothing leaks.
	defer func() {
		err = errors.Join(err, otelShutdown(context.Background()))
	}()

	flag.Parse()

	// Gracefully shutdown the gRPC server on SIGINT/SIGTERM
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	slog.Debug("Connecting to ClickHouse",
		slog.String("clickhouseAddr", *clickhouseAddr),
		slog.String("clickhouseDatabase", *clickhouseDatabase),
		slog.String("clickhouseUsername", *clickhouseUsername))
	store, err := NewClickHouseMetricsStore(ctx, *clickhouseAddr, *clickhouseDatabase, *clickhouseUsername, *clickhousePassword)
	if err != nil {
		return fmt.Errorf("connecting to clickhouse at %s: %w", *clickhouseAddr, err)
	}
	defer func() {
		err = errors.Join(err, store.Close())
	}()

	if err := store.CreateTables(ctx); err != nil {
		return fmt.Errorf("creating clickhouse tables: %w", err)
	}

	slog.Debug("Starting listener", slog.String("listenAddr", *listenAddr))
	listener, err := net.Listen("tcp", *listenAddr)
	if err != nil {
		return err
	}

	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.MaxRecvMsgSize(*maxReceiveMessageSize),
		grpc.Creds(insecure.NewCredentials()),
	)
	colmetricspb.RegisterMetricsServiceServer(grpcServer, newServer(*listenAddr, store))

	slog.Debug("Starting gRPC server")

	serveErr := make(chan error, 1)
	go func() { serveErr <- grpcServer.Serve(listener) }()

	select {
	case err = <-serveErr:
		return err
	case <-ctx.Done():
		slog.Info("Shutdown signal received, stopping gRPC server")
		stopped := make(chan struct{})
		go func() {
			grpcServer.GracefulStop()
			close(stopped)
		}()
		select {
		case <-stopped:
		case <-time.After(*shutdownTimeout):
			slog.Warn("Graceful shutdown timed out, forcing stop")
			grpcServer.Stop()
		}
		return nil
	}
}
