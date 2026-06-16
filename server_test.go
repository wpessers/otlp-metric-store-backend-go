package main

import (
	"context"
	"errors"
	"log"
	"net"
	"testing"

	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	otelmetrics "go.opentelemetry.io/proto/otlp/metrics/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func TestMetricsServiceServer_Export(t *testing.T) {
	ctx := context.Background()

	client, closer := server()
	defer closer()

	type expectation struct {
		out *colmetricspb.ExportMetricsServiceResponse
		err error
	}

	tests := map[string]struct {
		in       *colmetricspb.ExportMetricsServiceRequest
		expected expectation
	}{
		"Must_Success": {
			in: &colmetricspb.ExportMetricsServiceRequest{
				ResourceMetrics: []*otelmetrics.ResourceMetrics{
					{
						ScopeMetrics: []*otelmetrics.ScopeMetrics{},
						SchemaUrl:    "dash0.com/otlp-metrics-store-backend",
					},
				},
			},
			expected: expectation{
				out: &colmetricspb.ExportMetricsServiceResponse{},
				err: nil,
			},
		},
	}

	for scenario, tt := range tests {
		t.Run(scenario, func(t *testing.T) {
			out, err := client.Export(ctx, tt.in)
			if err != nil {
				if tt.expected.err.Error() != err.Error() {
					t.Errorf("Err -> \nWant: %q\nGot: %q\n", tt.expected.err, err)
				}
			} else {
				expectedPartialSuccess := tt.expected.out.GetPartialSuccess()
				if expectedPartialSuccess.GetRejectedDataPoints() != out.GetPartialSuccess().GetRejectedDataPoints() ||
					expectedPartialSuccess.GetErrorMessage() != out.GetPartialSuccess().GetErrorMessage() {
					t.Errorf("Out -> \nWant: %q\nGot : %q", tt.expected.out, out)
				}
			}

		})
	}
}

func server() (colmetricspb.MetricsServiceClient, func()) {
	addr := "localhost:4317"
	buffer := 101024 * 1024
	lis := bufconn.Listen(buffer)

	baseServer := grpc.NewServer()
	colmetricspb.RegisterMetricsServiceServer(baseServer, newServer(addr, nil))
	go func() {
		if err := baseServer.Serve(lis); err != nil {
			log.Printf("error serving server: %v", err)
		}
	}()

	conn, err := grpc.NewClient("passthrough://bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Printf("error connecting to server: %v", err)
	}

	closer := func() {
		err := lis.Close()
		if err != nil {
			log.Printf("error closing listener: %v", err)
		}
		baseServer.Stop()
	}

	client := colmetricspb.NewMetricsServiceClient(conn)

	return client, closer
}

// in-memory MetricsStore to mimic ClickHouse persistence
type fakeStore struct {
	gaugeCalls  int
	sumCalls    int
	seriesCalls int
	gaugeRows   []GaugeRow
	sumRows     []SumRow
	seriesRows  []MetricSeriesRow
	insertErr   error
}

func (f *fakeStore) CreateTables(context.Context) error { return nil }
func (f *fakeStore) Close() error                       { return nil }

func (f *fakeStore) InsertGauge(_ context.Context, rows []GaugeRow) error {
	f.gaugeCalls++
	if f.insertErr != nil {
		return f.insertErr
	}
	f.gaugeRows = append(f.gaugeRows, rows...)
	return nil
}

func (f *fakeStore) InsertSum(_ context.Context, rows []SumRow) error {
	f.sumCalls++
	if f.insertErr != nil {
		return f.insertErr
	}
	f.sumRows = append(f.sumRows, rows...)
	return nil
}

func (f *fakeStore) InsertSeries(_ context.Context, rows []MetricSeriesRow) error {
	f.seriesCalls++
	if f.insertErr != nil {
		return f.insertErr
	}
	f.seriesRows = append(f.seriesRows, rows...)
	return nil
}

func gaugeRequest(service, metric string, value float64) *colmetricspb.ExportMetricsServiceRequest {
	return &colmetricspb.ExportMetricsServiceRequest{
		ResourceMetrics: []*otelmetrics.ResourceMetrics{{
			Resource: &resourcepb.Resource{
				Attributes: []*commonpb.KeyValue{{
					Key:   "service.name",
					Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: service}},
				}},
			},
			ScopeMetrics: []*otelmetrics.ScopeMetrics{{
				Scope: &commonpb.InstrumentationScope{Name: "test-scope"},
				Metrics: []*otelmetrics.Metric{{
					Name: metric,
					Data: &otelmetrics.Metric_Gauge{Gauge: &otelmetrics.Gauge{
						DataPoints: []*otelmetrics.NumberDataPoint{{
							TimeUnixNano: 1,
							Value:        &otelmetrics.NumberDataPoint_AsDouble{AsDouble: value},
						}},
					}},
				}},
			}},
		}},
	}
}

func sumRequest(service, metric string, value int64, monotonic bool) *colmetricspb.ExportMetricsServiceRequest {
	return &colmetricspb.ExportMetricsServiceRequest{
		ResourceMetrics: []*otelmetrics.ResourceMetrics{{
			Resource: &resourcepb.Resource{
				Attributes: []*commonpb.KeyValue{{
					Key:   "service.name",
					Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: service}},
				}},
			},
			ScopeMetrics: []*otelmetrics.ScopeMetrics{{
				Scope: &commonpb.InstrumentationScope{Name: "test-scope"},
				Metrics: []*otelmetrics.Metric{{
					Name: metric,
					Data: &otelmetrics.Metric_Sum{Sum: &otelmetrics.Sum{
						AggregationTemporality: otelmetrics.AggregationTemporality_AGGREGATION_TEMPORALITY_CUMULATIVE,
						IsMonotonic:            monotonic,
						DataPoints: []*otelmetrics.NumberDataPoint{{
							TimeUnixNano: 1,
							Value:        &otelmetrics.NumberDataPoint_AsInt{AsInt: value},
						}},
					}},
				}},
			}},
		}},
	}
}

// Test forwarding mapped metric rows to the store
func TestMetricsServiceServer_Export_WithStore(t *testing.T) {
	ctx := context.Background()

	t.Run("forwards gauge data and series to the store", func(t *testing.T) {
		store := &fakeStore{}
		srv := newServer("test", store)

		resp, err := srv.Export(ctx, gaugeRequest("svc-a", "demo.gauge", 42.5))
		if err != nil {
			t.Fatalf("Export() error = %v", err)
		}
		if resp == nil {
			t.Fatal("Export() response is nil")
		}
		if store.gaugeCalls != 1 {
			t.Errorf("InsertGauge calls = %d, want 1", store.gaugeCalls)
		}
		if store.sumCalls != 0 {
			t.Errorf("InsertSum calls = %d, want 0", store.sumCalls)
		}
		if store.seriesCalls != 1 {
			t.Errorf("InsertSeries calls = %d, want 1", store.seriesCalls)
		}
		if len(store.gaugeRows) != 1 {
			t.Fatalf("gauge rows = %d, want 1", len(store.gaugeRows))
		}
		if got := store.gaugeRows[0].Value; got != 42.5 {
			t.Errorf("Value = %v, want %v", got, 42.5)
		}
		if len(store.seriesRows) != 1 {
			t.Fatalf("series rows = %d, want 1", len(store.seriesRows))
		}
		if store.seriesRows[0].SeriesID != store.gaugeRows[0].SeriesID {
			t.Errorf("series SeriesID (%d) != gauge SeriesID (%d)", store.seriesRows[0].SeriesID, store.gaugeRows[0].SeriesID)
		}
	})

	t.Run("forwards sum data to the store", func(t *testing.T) {
		store := &fakeStore{}
		srv := newServer("test", store)

		resp, err := srv.Export(ctx, sumRequest("svc-b", "demo.sum", 7, true))
		if err != nil {
			t.Fatalf("Export() error = %v", err)
		}
		if resp == nil {
			t.Fatal("Export() response is nil")
		}
		if store.sumCalls != 1 {
			t.Errorf("InsertSum calls = %d, want 1", store.sumCalls)
		}
		if store.gaugeCalls != 0 {
			t.Errorf("InsertGauge calls = %d, want 0", store.gaugeCalls)
		}
		if len(store.sumRows) != 1 {
			t.Fatalf("sum rows = %d, want 1", len(store.sumRows))
		}
		if got := store.sumRows[0].Value; got != 7 {
			t.Errorf("Value = %v, want %v", got, float64(7))
		}
		if store.seriesCalls != 1 {
			t.Errorf("InsertSeries calls = %d, want 1", store.seriesCalls)
		}
		if len(store.seriesRows) != 1 {
			t.Fatalf("series rows = %d, want 1", len(store.seriesRows))
		}
		if store.seriesRows[0].SeriesID != store.sumRows[0].SeriesID {
			t.Errorf("series SeriesID (%d) != sum SeriesID (%d)", store.seriesRows[0].SeriesID, store.sumRows[0].SeriesID)
		}
	})

	t.Run("no insert calls for an empty request", func(t *testing.T) {
		store := &fakeStore{}
		srv := newServer("test", store)

		resp, err := srv.Export(ctx, &colmetricspb.ExportMetricsServiceRequest{})
		if err != nil {
			t.Fatalf("Export() error = %v", err)
		}
		if resp == nil {
			t.Fatal("Export() response is nil")
		}
		if store.gaugeCalls != 0 || store.sumCalls != 0 || store.seriesCalls != 0 {
			t.Errorf("insert calls = (gauge %d, sum %d, series %d), want (0, 0, 0)", store.gaugeCalls, store.sumCalls, store.seriesCalls)
		}
	})

	t.Run("propagates a store insert error", func(t *testing.T) {
		wantErr := errors.New("insert failed")
		store := &fakeStore{insertErr: wantErr}
		srv := newServer("test", store)

		resp, err := srv.Export(ctx, gaugeRequest("svc-a", "demo.gauge", 1))
		if !errors.Is(err, wantErr) {
			t.Errorf("Export() error = %v, want %v", err, wantErr)
		}
		if resp != nil {
			t.Errorf("Export() response = %v, want nil on error", resp)
		}
	})
}
