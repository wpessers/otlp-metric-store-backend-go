package main

import (
	"context"
	"fmt"
	"log/slog"

	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
)

type dash0MetricsServiceServer struct {
	addr  string
	store MetricsStore

	colmetricspb.UnimplementedMetricsServiceServer
}

func newServer(addr string, store MetricsStore) colmetricspb.MetricsServiceServer {
	return &dash0MetricsServiceServer{addr: addr, store: store}
}

func (m *dash0MetricsServiceServer) Export(ctx context.Context, request *colmetricspb.ExportMetricsServiceRequest) (*colmetricspb.ExportMetricsServiceResponse, error) {
	slog.DebugContext(ctx, "Received ExportMetricsServiceRequest")
	metricsReceivedCounter.Add(ctx, 1)

	// A nil store means no persistent store was configured. We want to return early here.
	if m.store == nil {
		slog.WarnContext(ctx, "metrics store is nil, dropping metrics (no persistence configured)")
		return &colmetricspb.ExportMetricsServiceResponse{}, nil
	}

	mapped, err := MapMetrics(request.GetResourceMetrics())
	if err != nil {
		slog.ErrorContext(ctx, "failed to map metrics", slog.Any("error", err))
		return nil, err
	}

	if len(mapped.Series) > 0 {
		if err := m.store.InsertSeries(ctx, mapped.Series); err != nil {
			return nil, fmt.Errorf("inserting metric series: %w", err)
		}
	}
	if len(mapped.Gauges) > 0 {
		if err := m.store.InsertGauge(ctx, mapped.Gauges); err != nil {
			return nil, fmt.Errorf("inserting gauge points: %w", err)
		}
	}
	if len(mapped.Sums) > 0 {
		if err := m.store.InsertSum(ctx, mapped.Sums); err != nil {
			return nil, fmt.Errorf("inserting sum points: %w", err)
		}
	}

	return &colmetricspb.ExportMetricsServiceResponse{}, nil
}
