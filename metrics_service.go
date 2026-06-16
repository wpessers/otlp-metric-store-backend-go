package main

import (
	"context"
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

	rm := request.GetResourceMetrics()

	if gaugeRows := MapGaugeRows(rm); len(gaugeRows) > 0 {
		if err := m.store.InsertGauge(ctx, gaugeRows); err != nil {
			return nil, err
		}
	}
	if sumRows := MapSumRows(rm); len(sumRows) > 0 {
		if err := m.store.InsertSum(ctx, sumRows); err != nil {
			return nil, err
		}
	}

	return &colmetricspb.ExportMetricsServiceResponse{}, nil
}
