package main

import (
	"maps"
	"testing"

	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
)

func baseGaugeMetadata() MetricMetadata {
	return MetricMetadata{
		MetricType:        "gauge",
		ServiceName:       "checkout",
		MetricName:        "http.server.duration",
		MetricDescription: "HTTP server request duration",
		MetricUnit:        "ms",

		ResourceSchemaUrl: "https://opentelemetry.io/schemas/1.25.0",
		ResourceAttributes: map[string]string{
			"service.name": "checkout",
			"host.name":    "host-a",
			"k8s.pod.name": "checkout-123",
		},

		ScopeName:      "go.opentelemetry.io/otel/sdk",
		ScopeVersion:   "1.37.0",
		ScopeSchemaUrl: "https://opentelemetry.io/schemas/1.25.0",
		ScopeAttributes: map[string]string{
			"scope.attr.a": "a",
			"scope.attr.b": "b",
		},
		ScopeDroppedAttrCount: 0,

		Attributes: map[string]string{
			"http.request.method":       "GET",
			"http.route":                "/checkout",
			"http.response.status_code": "200",
		},
	}
}

func baseSumMetadata() MetricMetadata {
	m := baseGaugeMetadata()
	m.MetricType = "sum"
	m.MetricName = "http.server.requests"
	m.MetricUnit = "{request}"
	m.MetricDescription = "HTTP server request count"
	m.AggregationTemporality = int32(metricspb.AggregationTemporality_AGGREGATION_TEMPORALITY_CUMULATIVE)
	m.IsMonotonic = true
	return m
}

// cloneMetadata returns a deep copy of m with independent attribute maps
func cloneMetadata(m MetricMetadata) MetricMetadata {
	m.ResourceAttributes = maps.Clone(m.ResourceAttributes)
	m.ScopeAttributes = maps.Clone(m.ScopeAttributes)
	m.Attributes = maps.Clone(m.Attributes)
	return m
}

// mustSeriesID computes the SeriesID for metadata, failing the test on error.
func mustSeriesID(t *testing.T, metadata MetricMetadata) uint64 {
	t.Helper()

	id, err := metadata.SeriesID()
	if err != nil {
		t.Fatalf("SeriesID() returned error: %v", err)
	}

	return id
}

func TestMetricMetadataSeriesIDIsDeterministic(t *testing.T) {
	cases := map[string]MetricMetadata{
		"gauge": baseGaugeMetadata(),
		"sum":   baseSumMetadata(),
	}

	for name, metadata := range cases {
		t.Run(name, func(t *testing.T) {
			first := mustSeriesID(t, metadata)
			second := mustSeriesID(t, metadata)

			if first != second {
				t.Fatalf("expected deterministic series ID, got %d and %d", first, second)
			}
		})
	}
}

func TestMetricMetadataSeriesIDIgnoresAttributeOrder(t *testing.T) {
	a := baseGaugeMetadata()
	b := cloneMetadata(a)
	b.Attributes = map[string]string{
		"http.response.status_code": "200",
		"http.route":                "/checkout",
		"http.request.method":       "GET",
	}

	expectedID := mustSeriesID(t, a)
	actualID := mustSeriesID(t, b)

	if expectedID != actualID {
		t.Fatalf("expected same series ID for reordered data point attributes, got %d and %d", expectedID, actualID)
	}
}

func TestMetricMetadataSeriesIDIgnoresResourceAttributeOrder(t *testing.T) {
	a := baseGaugeMetadata()
	b := cloneMetadata(a)
	b.ResourceAttributes = map[string]string{
		"k8s.pod.name": "checkout-123",
		"host.name":    "host-a",
		"service.name": "checkout",
	}

	expectedID := mustSeriesID(t, a)
	actualID := mustSeriesID(t, b)

	if expectedID != actualID {
		t.Fatalf("expected same series ID for reordered resource attributes, got %d and %d", expectedID, actualID)
	}
}

func TestMetricMetadataSeriesIDIgnoresScopeAttributeOrder(t *testing.T) {
	a := baseGaugeMetadata()
	b := cloneMetadata(a)
	b.ScopeAttributes = map[string]string{
		"scope.attr.b": "b",
		"scope.attr.a": "a",
	}

	expectedID := mustSeriesID(t, a)
	actualID := mustSeriesID(t, b)

	if expectedID != actualID {
		t.Fatalf("expected same series ID for reordered scope attributes, got %d and %d", expectedID, actualID)
	}
}

func TestMetricMetadataSeriesIDChangesWithIdentityFields(t *testing.T) {
	base := baseSumMetadata()
	baseID := mustSeriesID(t, base)

	cases := map[string]func(m *MetricMetadata){
		"metric type":          func(m *MetricMetadata) { m.MetricType = "gauge" },
		"metric name":          func(m *MetricMetadata) { m.MetricName = "http.client.duration" },
		"metric unit":          func(m *MetricMetadata) { m.MetricUnit = "s" },
		"resource attribute":   func(m *MetricMetadata) { m.ResourceAttributes["host.name"] = "host-b" },
		"resource schema url":  func(m *MetricMetadata) { m.ResourceSchemaUrl = "https://opentelemetry.io/schemas/1.26.0" },
		"scope name":           func(m *MetricMetadata) { m.ScopeName = "custom-scope" },
		"scope version":        func(m *MetricMetadata) { m.ScopeVersion = "2.0.0" },
		"scope attribute":      func(m *MetricMetadata) { m.ScopeAttributes["scope.attr.a"] = "z" },
		"scope schema url":     func(m *MetricMetadata) { m.ScopeSchemaUrl = "https://opentelemetry.io/schemas/1.26.0/scope" },
		"data point attribute": func(m *MetricMetadata) { m.Attributes["http.route"] = "/cart" },
		"aggregation temporality": func(m *MetricMetadata) {
			m.AggregationTemporality = int32(metricspb.AggregationTemporality_AGGREGATION_TEMPORALITY_DELTA)
		},
		"is monotonic": func(m *MetricMetadata) { m.IsMonotonic = false },
	}

	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			m := cloneMetadata(base)
			mutate(&m)

			if got := mustSeriesID(t, m); got == baseID {
				t.Fatalf("expected a different series ID after changing %s, got %d for both", name, baseID)
			}
		})
	}
}

func TestMetricMetadataSeriesIDIgnoresNonIdentityFields(t *testing.T) {
	base := baseSumMetadata()
	baseID := mustSeriesID(t, base)

	cases := map[string]func(m *MetricMetadata){
		"service name":             func(m *MetricMetadata) { m.ServiceName = "payments" },
		"metric description":       func(m *MetricMetadata) { m.MetricDescription = "a reworded description" },
		"scope dropped attr count": func(m *MetricMetadata) { m.ScopeDroppedAttrCount = 5 },
	}

	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			m := cloneMetadata(base)
			mutate(&m)

			if got := mustSeriesID(t, m); got != baseID {
				t.Fatalf("expected the same series ID after changing %s, got %d and %d", name, baseID, got)
			}
		})
	}
}

func TestMetricMetadataSeriesIDTreatsNilAndEmptyAttributesEqual(t *testing.T) {
	withNil := baseGaugeMetadata()
	withNil.ResourceAttributes = nil
	withNil.ScopeAttributes = nil
	withNil.Attributes = nil

	withEmpty := baseGaugeMetadata()
	withEmpty.ResourceAttributes = map[string]string{}
	withEmpty.ScopeAttributes = map[string]string{}
	withEmpty.Attributes = map[string]string{}

	nilID := mustSeriesID(t, withNil)
	emptyID := mustSeriesID(t, withEmpty)

	if nilID != emptyID {
		t.Fatalf("expected nil and empty attribute maps to produce the same series ID, got %d and %d", nilID, emptyID)
	}
}
