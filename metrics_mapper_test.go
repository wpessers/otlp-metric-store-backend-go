package main

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
)

// gaugeResourceMetrics returns a fully populated gauge fixture.
func gaugeResourceMetrics() []*metricspb.ResourceMetrics {
	return []*metricspb.ResourceMetrics{{
		Resource: &resourcepb.Resource{
			Attributes: []*commonpb.KeyValue{
				{Key: "service.name", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "checkout"}}},
				{Key: "host.name", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "node-1"}}},
			},
		},
		SchemaUrl: "https://opentelemetry.io/schemas/1.21.0",
		ScopeMetrics: []*metricspb.ScopeMetrics{{
			Scope: &commonpb.InstrumentationScope{
				Name:                   "runtime-instrumentation",
				Version:                "1.2.3",
				Attributes:             []*commonpb.KeyValue{{Key: "scope.team", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "platform"}}}},
				DroppedAttributesCount: 2,
			},
			SchemaUrl: "https://opentelemetry.io/schemas/1.21.0/scope",
			Metrics: []*metricspb.Metric{{
				Name:        "memory.usage",
				Description: "Process memory usage",
				Unit:        "By",
				Data: &metricspb.Metric_Gauge{Gauge: &metricspb.Gauge{
					DataPoints: []*metricspb.NumberDataPoint{{
						Attributes:        []*commonpb.KeyValue{{Key: "state", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "used"}}}},
						StartTimeUnixNano: 1_000_000_000,
						TimeUnixNano:      2_000_000_000,
						Value:             &metricspb.NumberDataPoint_AsDouble{AsDouble: 512.0},
						Flags:             0,
					}},
				}},
			}},
		}},
	}}
}

// sumResourceMetrics returns a fully populated sum fixture.
func sumResourceMetrics() []*metricspb.ResourceMetrics {
	return []*metricspb.ResourceMetrics{{
		Resource: &resourcepb.Resource{
			Attributes: []*commonpb.KeyValue{
				{Key: "service.name", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "checkout"}}},
				{Key: "host.name", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "node-1"}}},
			},
		},
		SchemaUrl: "https://opentelemetry.io/schemas/1.21.0",
		ScopeMetrics: []*metricspb.ScopeMetrics{{
			Scope: &commonpb.InstrumentationScope{
				Name:                   "runtime-instrumentation",
				Version:                "1.2.3",
				Attributes:             []*commonpb.KeyValue{{Key: "scope.team", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "platform"}}}},
				DroppedAttributesCount: 2,
			},
			SchemaUrl: "https://opentelemetry.io/schemas/1.21.0/scope",
			Metrics: []*metricspb.Metric{{
				Name:        "http.requests",
				Description: "Total HTTP requests",
				Unit:        "{request}",
				Data: &metricspb.Metric_Sum{Sum: &metricspb.Sum{
					AggregationTemporality: metricspb.AggregationTemporality_AGGREGATION_TEMPORALITY_CUMULATIVE,
					IsMonotonic:            true,
					DataPoints: []*metricspb.NumberDataPoint{{
						Attributes:        []*commonpb.KeyValue{{Key: "method", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "GET"}}}},
						StartTimeUnixNano: 3_000_000_000,
						TimeUnixNano:      4_000_000_000,
						Value:             &metricspb.NumberDataPoint_AsInt{AsInt: 1234},
					}},
				}},
			}},
		}},
	}}
}

func TestMapMetrics_Gauge(t *testing.T) {
	m, err := MapMetrics(gaugeResourceMetrics())
	if err != nil {
		t.Fatalf("MapMetrics() error = %v", err)
	}

	t.Run("produces one series row and one gauge row", func(t *testing.T) {
		if len(m.Series) != 1 {
			t.Errorf("series rows = %d, want 1", len(m.Series))
		}
		if len(m.Gauges) != 1 {
			t.Errorf("gauge rows = %d, want 1", len(m.Gauges))
		}
		if len(m.Sums) != 0 {
			t.Errorf("sum rows = %d, want 0", len(m.Sums))
		}
	})

	t.Run("metadata lives on the series row", func(t *testing.T) {
		if len(m.Series) != 1 {
			t.Fatalf("series rows = %d, want 1", len(m.Series))
		}
		got := m.Series[0].MetricMetadata
		want := MetricMetadata{
			MetricType:            metricTypeGauge,
			ServiceName:           "checkout",
			MetricName:            "memory.usage",
			MetricDescription:     "Process memory usage",
			MetricUnit:            "By",
			ResourceAttributes:    map[string]string{"service.name": "checkout", "host.name": "node-1"},
			ResourceSchemaUrl:     "https://opentelemetry.io/schemas/1.21.0",
			ScopeName:             "runtime-instrumentation",
			ScopeVersion:          "1.2.3",
			ScopeAttributes:       map[string]string{"scope.team": "platform"},
			ScopeDroppedAttrCount: 2,
			ScopeSchemaUrl:        "https://opentelemetry.io/schemas/1.21.0/scope",
			Attributes:            map[string]string{"state": "used"},
			// AggregationTemporality and IsMonotonic are not set for gauges.
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("series metadata mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("point values live on the gauge row", func(t *testing.T) {
		if len(m.Gauges) != 1 {
			t.Fatalf("gauge rows = %d, want 1", len(m.Gauges))
		}
		got := m.Gauges[0]
		if got.Value != 512.0 {
			t.Errorf("Value = %v, want 512", got.Value)
		}
		if want := time.Unix(0, 1_000_000_000); !got.StartTimeUnix.Equal(want) {
			t.Errorf("StartTimeUnix = %v, want %v", got.StartTimeUnix, want)
		}
		if want := time.Unix(0, 2_000_000_000); !got.TimeUnix.Equal(want) {
			t.Errorf("TimeUnix = %v, want %v", got.TimeUnix, want)
		}
		if got.Flags != 0 {
			t.Errorf("Flags = %d, want 0", got.Flags)
		}
	})

	t.Run("series and point share the same SeriesID", func(t *testing.T) {
		if len(m.Series) != 1 || len(m.Gauges) != 1 {
			t.Fatalf("series rows = %d, gauge rows = %d, want 1 each", len(m.Series), len(m.Gauges))
		}
		if m.Series[0].SeriesID != m.Gauges[0].SeriesID {
			t.Errorf("series SeriesID (%d) != gauge SeriesID (%d)", m.Series[0].SeriesID, m.Gauges[0].SeriesID)
		}
	})
}

func TestMapMetrics_Sum(t *testing.T) {
	m, err := MapMetrics(sumResourceMetrics())
	if err != nil {
		t.Fatalf("MapMetrics() error = %v", err)
	}

	t.Run("produces one series row and one sum row", func(t *testing.T) {
		if len(m.Series) != 1 {
			t.Errorf("series rows = %d, want 1", len(m.Series))
		}
		if len(m.Sums) != 1 {
			t.Errorf("sum rows = %d, want 1", len(m.Sums))
		}
		if len(m.Gauges) != 0 {
			t.Errorf("gauge rows = %d, want 0", len(m.Gauges))
		}
	})

	t.Run("metadata lives on the series row", func(t *testing.T) {
		if len(m.Series) != 1 {
			t.Fatalf("series rows = %d, want 1", len(m.Series))
		}
		got := m.Series[0].MetricMetadata
		want := MetricMetadata{
			MetricType:             metricTypeSum,
			ServiceName:            "checkout",
			MetricName:             "http.requests",
			MetricDescription:      "Total HTTP requests",
			MetricUnit:             "{request}",
			ResourceAttributes:     map[string]string{"service.name": "checkout", "host.name": "node-1"},
			ResourceSchemaUrl:      "https://opentelemetry.io/schemas/1.21.0",
			ScopeName:              "runtime-instrumentation",
			ScopeVersion:           "1.2.3",
			ScopeAttributes:        map[string]string{"scope.team": "platform"},
			ScopeDroppedAttrCount:  2,
			ScopeSchemaUrl:         "https://opentelemetry.io/schemas/1.21.0/scope",
			Attributes:             map[string]string{"method": "GET"},
			AggregationTemporality: int32(metricspb.AggregationTemporality_AGGREGATION_TEMPORALITY_CUMULATIVE),
			IsMonotonic:            true,
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("series metadata mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("point values live on the sum row", func(t *testing.T) {
		if len(m.Sums) != 1 {
			t.Fatalf("sum rows = %d, want 1", len(m.Sums))
		}
		got := m.Sums[0]
		if got.Value != 1234 {
			t.Errorf("Value = %v, want 1234", got.Value)
		}
		if want := time.Unix(0, 3_000_000_000); !got.StartTimeUnix.Equal(want) {
			t.Errorf("StartTimeUnix = %v, want %v", got.StartTimeUnix, want)
		}
		if want := time.Unix(0, 4_000_000_000); !got.TimeUnix.Equal(want) {
			t.Errorf("TimeUnix = %v, want %v", got.TimeUnix, want)
		}
	})

	t.Run("series and point share the same SeriesID", func(t *testing.T) {
		if len(m.Series) != 1 || len(m.Sums) != 1 {
			t.Fatalf("series rows = %d, sum rows = %d, want 1 each", len(m.Series), len(m.Sums))
		}
		if m.Series[0].SeriesID != m.Sums[0].SeriesID {
			t.Errorf("series SeriesID (%d) != sum SeriesID (%d)", m.Series[0].SeriesID, m.Sums[0].SeriesID)
		}
	})
}

func TestMapMetrics_EmptyInput(t *testing.T) {
	m, err := MapMetrics(nil)
	if err != nil {
		t.Fatalf("MapMetrics() error = %v", err)
	}
	if len(m.Series) != 0 || len(m.Gauges) != 0 || len(m.Sums) != 0 {
		t.Errorf("MapMetrics(nil) = %+v, want all empty", m)
	}
}

func TestMapMetrics_RejectsZeroTimestamp(t *testing.T) {
	attr := func(k, v string) []*commonpb.KeyValue {
		return []*commonpb.KeyValue{{Key: k, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: v}}}}
	}

	rm := []*metricspb.ResourceMetrics{{
		Resource: &resourcepb.Resource{Attributes: attr("service.name", "checkout")},
		ScopeMetrics: []*metricspb.ScopeMetrics{{
			Scope: &commonpb.InstrumentationScope{Name: "scope"},
			Metrics: []*metricspb.Metric{
				{
					Name: "memory.usage",
					Data: &metricspb.Metric_Gauge{Gauge: &metricspb.Gauge{
						DataPoints: []*metricspb.NumberDataPoint{
							{TimeUnixNano: 2_000_000_000, Value: &metricspb.NumberDataPoint_AsDouble{AsDouble: 1}},
							{TimeUnixNano: 0, Value: &metricspb.NumberDataPoint_AsDouble{AsDouble: 2}},
						},
					}},
				},
				{
					Name: "http.requests",
					Data: &metricspb.Metric_Sum{Sum: &metricspb.Sum{
						DataPoints: []*metricspb.NumberDataPoint{
							{TimeUnixNano: 0, Value: &metricspb.NumberDataPoint_AsInt{AsInt: 5}},
						},
					}},
				},
			},
		}},
	}}

	m, err := MapMetrics(rm)
	if err != nil {
		t.Fatalf("MapMetrics() error = %v", err)
	}

	if len(m.Gauges) != 1 {
		t.Errorf("gauge rows = %d, want 1 (only the valid point is stored)", len(m.Gauges))
	}
	if len(m.Sums) != 0 {
		t.Errorf("sum rows = %d, want 0 (its only point is rejected)", len(m.Sums))
	}
	if m.RejectedDataPoints != 2 {
		t.Errorf("RejectedDataPoints = %d, want 2", m.RejectedDataPoints)
	}
	if m.UnsupportedMetrics != 0 {
		t.Errorf("UnsupportedMetrics = %d, want 0", m.UnsupportedMetrics)
	}
	// The sum's only point was rejected, so no orphan series row should exist for it.
	if len(m.Series) != 1 {
		t.Errorf("series rows = %d, want 1 (no series for a fully rejected metric)", len(m.Series))
	}
}

func TestMapMetrics_RejectsUnsupportedTypes(t *testing.T) {
	histogramDP := []*metricspb.HistogramDataPoint{{TimeUnixNano: 1}, {TimeUnixNano: 2}}
	expHistogramDP := []*metricspb.ExponentialHistogramDataPoint{{TimeUnixNano: 1}}
	summaryDP := []*metricspb.SummaryDataPoint{{TimeUnixNano: 1}, {TimeUnixNano: 2}, {TimeUnixNano: 3}}

	rm := []*metricspb.ResourceMetrics{{
		Resource: &resourcepb.Resource{},
		ScopeMetrics: []*metricspb.ScopeMetrics{{
			Scope: &commonpb.InstrumentationScope{Name: "scope"},
			Metrics: []*metricspb.Metric{
				{Name: "latency.histogram", Data: &metricspb.Metric_Histogram{Histogram: &metricspb.Histogram{DataPoints: histogramDP}}},
				{Name: "latency.exphistogram", Data: &metricspb.Metric_ExponentialHistogram{ExponentialHistogram: &metricspb.ExponentialHistogram{DataPoints: expHistogramDP}}},
				{Name: "latency.summary", Data: &metricspb.Metric_Summary{Summary: &metricspb.Summary{DataPoints: summaryDP}}},
			},
		}},
	}}

	m, err := MapMetrics(rm)
	if err != nil {
		t.Fatalf("MapMetrics() error = %v", err)
	}

	if len(m.Series) != 0 || len(m.Gauges) != 0 || len(m.Sums) != 0 {
		t.Errorf("unsupported metrics produced rows: series=%d gauges=%d sums=%d, want 0 each", len(m.Series), len(m.Gauges), len(m.Sums))
	}
	if m.UnsupportedMetrics != 3 {
		t.Errorf("UnsupportedMetrics = %d, want 3", m.UnsupportedMetrics)
	}
	// 2 histogram + 1 exponential histogram + 3 summary data points.
	if m.RejectedDataPoints != 6 {
		t.Errorf("RejectedDataPoints = %d, want 6", m.RejectedDataPoints)
	}
}

func TestMapMetrics_DeduplicatesSeries(t *testing.T) {
	attr := func(k, v string) []*commonpb.KeyValue {
		return []*commonpb.KeyValue{{Key: k, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: v}}}}
	}

	rm := []*metricspb.ResourceMetrics{{
		Resource: &resourcepb.Resource{
			Attributes: attr("service.name", "checkout"),
		},
		ScopeMetrics: []*metricspb.ScopeMetrics{{
			Scope: &commonpb.InstrumentationScope{Name: "scope"},
			Metrics: []*metricspb.Metric{{
				Name: "memory.usage",
				Data: &metricspb.Metric_Gauge{Gauge: &metricspb.Gauge{
					DataPoints: []*metricspb.NumberDataPoint{
						{Attributes: attr("state", "used"), TimeUnixNano: 1, Value: &metricspb.NumberDataPoint_AsDouble{AsDouble: 1}},
						{Attributes: attr("state", "used"), TimeUnixNano: 2, Value: &metricspb.NumberDataPoint_AsDouble{AsDouble: 2}},
						{Attributes: attr("state", "free"), TimeUnixNano: 3, Value: &metricspb.NumberDataPoint_AsDouble{AsDouble: 3}},
					},
				}},
			}},
		}},
	}}

	m, err := MapMetrics(rm)
	if err != nil {
		t.Fatalf("MapMetrics() error = %v", err)
	}

	if len(m.Gauges) != 3 {
		t.Errorf("gauge rows = %d, want 3 (one per data point)", len(m.Gauges))
	}
	if len(m.Series) != 2 {
		t.Fatalf("series rows = %d, want 2 (deduplicated by identity)", len(m.Series))
	}
	if m.Gauges[0].SeriesID != m.Gauges[1].SeriesID {
		t.Errorf("identical data points got different series IDs: %d and %d", m.Gauges[0].SeriesID, m.Gauges[1].SeriesID)
	}
	if m.Gauges[0].SeriesID == m.Gauges[2].SeriesID {
		t.Errorf("data points with different attributes share series ID %d", m.Gauges[0].SeriesID)
	}
}
