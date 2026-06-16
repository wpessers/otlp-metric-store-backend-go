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
	m := MapMetrics(gaugeResourceMetrics())

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

	t.Run("series and point share the placeholder SeriesID", func(t *testing.T) {
		if len(m.Series) != 1 || len(m.Gauges) != 1 {
			t.Fatalf("series rows = %d, gauge rows = %d, want 1 each", len(m.Series), len(m.Gauges))
		}
		if m.Series[0].SeriesID != placeholderSeriesID {
			t.Errorf("series SeriesID = %d, want %d", m.Series[0].SeriesID, placeholderSeriesID)
		}
		if m.Gauges[0].SeriesID != placeholderSeriesID {
			t.Errorf("gauge SeriesID = %d, want %d", m.Gauges[0].SeriesID, placeholderSeriesID)
		}
		if m.Series[0].SeriesID != m.Gauges[0].SeriesID {
			t.Errorf("series SeriesID (%d) != gauge SeriesID (%d)", m.Series[0].SeriesID, m.Gauges[0].SeriesID)
		}
	})
}

func TestMapMetrics_Sum(t *testing.T) {
	m := MapMetrics(sumResourceMetrics())

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

	t.Run("series and point share the placeholder SeriesID", func(t *testing.T) {
		if len(m.Series) != 1 || len(m.Sums) != 1 {
			t.Fatalf("series rows = %d, sum rows = %d, want 1 each", len(m.Series), len(m.Sums))
		}
		if m.Series[0].SeriesID != placeholderSeriesID {
			t.Errorf("series SeriesID = %d, want %d", m.Series[0].SeriesID, placeholderSeriesID)
		}
		if m.Sums[0].SeriesID != placeholderSeriesID {
			t.Errorf("sum SeriesID = %d, want %d", m.Sums[0].SeriesID, placeholderSeriesID)
		}
		if m.Series[0].SeriesID != m.Sums[0].SeriesID {
			t.Errorf("series SeriesID (%d) != sum SeriesID (%d)", m.Series[0].SeriesID, m.Sums[0].SeriesID)
		}
	})
}

func TestMapMetrics_EmptyInput(t *testing.T) {
	m := MapMetrics(nil)
	if len(m.Series) != 0 || len(m.Gauges) != 0 || len(m.Sums) != 0 {
		t.Errorf("MapMetrics(nil) = %+v, want all empty", m)
	}
}
