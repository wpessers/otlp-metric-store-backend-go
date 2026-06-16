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
	exportRequestsCounter.Add(ctx, 1)

	// A nil store means no persistent store was configured. We want to return early here.
	if m.store == nil {
		slog.WarnContext(ctx, "metrics store is nil, dropping metrics (no persistence configured)")
		return &colmetricspb.ExportMetricsServiceResponse{}, nil
	}

	mapped, err := MapMetrics(request.GetResourceMetrics())
	if err != nil {
		slog.ErrorContext(ctx, "failed to map metrics", slog.Any("error", err))
		exportFailuresCounter.Add(ctx, 1)
		return nil, fmt.Errorf("mapping metrics: %w", err)
	}
	seriesRowsCounter.Add(ctx, int64(len(mapped.Series)))
	gaugePointsCounter.Add(ctx, int64(len(mapped.Gauges)))
	sumPointsCounter.Add(ctx, int64(len(mapped.Sums)))

	if len(mapped.Series) > 0 {
		if err := m.store.InsertSeries(ctx, mapped.Series); err != nil {
			logMetricExportFailure(ctx, "failed to insert metric series", mapped, err)
			exportFailuresCounter.Add(ctx, 1)
			return nil, fmt.Errorf("inserting metric series: %w", err)
		}
	}
	if len(mapped.Gauges) > 0 {
		if err := m.store.InsertGauge(ctx, mapped.Gauges); err != nil {
			logMetricExportFailure(ctx, "failed to insert gauge points", mapped, err)
			exportFailuresCounter.Add(ctx, 1)
			return nil, fmt.Errorf("inserting gauge points: %w", err)
		}
	}
	if len(mapped.Sums) > 0 {
		if err := m.store.InsertSum(ctx, mapped.Sums); err != nil {
			logMetricExportFailure(ctx, "failed to insert sum points", mapped, err)
			exportFailuresCounter.Add(ctx, 1)
			return nil, fmt.Errorf("inserting sum points: %w", err)
		}
	}

	slog.DebugContext(ctx, "processed metric export request", metricExportLogAttributes(mapped)...)
	return &colmetricspb.ExportMetricsServiceResponse{}, nil
}

func logMetricExportFailure(ctx context.Context, message string, mapped MappedMetrics, err error) {
	attrs := append(metricExportLogAttributes(mapped), slog.Any("error", err))
	slog.ErrorContext(ctx, message, attrs...)
}

func metricExportLogAttributes(mapped MappedMetrics) []any {
	return []any{
		slog.Int("series_rows", len(mapped.Series)),
		slog.Int("gauge_points", len(mapped.Gauges)),
		slog.Int("sum_points", len(mapped.Sums)),
	}
}
