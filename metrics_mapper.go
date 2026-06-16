package main

import (
	"fmt"
	"time"

	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
)

const (
	metricTypeGauge = "gauge"
	metricTypeSum   = "sum"
)

// serviceName extracts the service.name from resource attributes, returning "" if not found.
func serviceName(resource *resourcepb.Resource) string {
	if resource == nil {
		return ""
	}
	for _, attr := range resource.GetAttributes() {
		if attr.GetKey() == "service.name" {
			return attr.GetValue().GetStringValue()
		}
	}
	return ""
}

// kvToMap converts a slice of OTLP KeyValue pairs to a Go map.
func kvToMap(attrs []*commonpb.KeyValue) map[string]string {
	m := make(map[string]string, len(attrs))
	for _, kv := range attrs {
		m[kv.GetKey()] = anyValueToString(kv.GetValue())
	}
	return m
}

// anyValueToString converts an OTLP AnyValue to its string representation.
func anyValueToString(v *commonpb.AnyValue) string {
	if v == nil {
		return ""
	}
	switch v.Value.(type) {
	case *commonpb.AnyValue_StringValue:
		return v.GetStringValue()
	case *commonpb.AnyValue_IntValue:
		return fmt.Sprintf("%d", v.GetIntValue())
	case *commonpb.AnyValue_DoubleValue:
		return fmt.Sprintf("%g", v.GetDoubleValue())
	case *commonpb.AnyValue_BoolValue:
		return fmt.Sprintf("%t", v.GetBoolValue())
	default:
		return fmt.Sprintf("%v", v)
	}
}

// nanosToTime converts a uint64 nanoseconds-since-epoch to time.Time.
func nanosToTime(nanos uint64) time.Time {
	return time.Unix(0, int64(nanos))
}

// numberDataPointValue extracts the float64 value from a NumberDataPoint.
func numberDataPointValue(dp *metricspb.NumberDataPoint) float64 {
	switch v := dp.GetValue().(type) {
	case *metricspb.NumberDataPoint_AsDouble:
		return v.AsDouble
	case *metricspb.NumberDataPoint_AsInt:
		return float64(v.AsInt)
	default:
		return 0
	}
}

// MappedMetrics holds the rows extracted from one export request
type MappedMetrics struct {
	Series []MetricSeriesRow
	Gauges []GaugeRow
	Sums   []SumRow
}

// MapMetrics walks the request and produces the rows to persist: a deduplicated
// series row per unique metric identity, and a gauge or sum data-point row for each datapoint
func MapMetrics(resourceMetrics []*metricspb.ResourceMetrics) (MappedMetrics, error) {
	var mapped MappedMetrics
	seen := make(map[uint64]struct{})

	for _, rm := range resourceMetrics {
		svcName := serviceName(rm.GetResource())
		resAttrs := kvToMap(rm.GetResource().GetAttributes())
		resSchemaUrl := rm.GetSchemaUrl()

		for _, sm := range rm.GetScopeMetrics() {
			scope := sm.GetScope()
			scopeAttrs := kvToMap(scope.GetAttributes())

			for _, metric := range sm.GetMetrics() {
				switch data := metric.GetData().(type) {
				case *metricspb.Metric_Gauge:
					for _, dp := range data.Gauge.GetDataPoints() {
						meta := MetricMetadata{
							MetricType:            metricTypeGauge,
							ServiceName:           svcName,
							MetricName:            metric.GetName(),
							MetricDescription:     metric.GetDescription(),
							MetricUnit:            metric.GetUnit(),
							ResourceAttributes:    resAttrs,
							ResourceSchemaUrl:     resSchemaUrl,
							ScopeName:             scope.GetName(),
							ScopeVersion:          scope.GetVersion(),
							ScopeAttributes:       scopeAttrs,
							ScopeDroppedAttrCount: scope.GetDroppedAttributesCount(),
							ScopeSchemaUrl:        sm.GetSchemaUrl(),
							Attributes:            kvToMap(dp.GetAttributes()),
						}
						id, err := mapped.recordSeries(seen, meta)
						if err != nil {
							return MappedMetrics{}, err
						}
						mapped.Gauges = append(mapped.Gauges, GaugeRow{
							SeriesID:        id,
							NumberDataPoint: numberDataPoint(dp),
						})
					}
				case *metricspb.Metric_Sum:
					sum := data.Sum
					for _, dp := range sum.GetDataPoints() {
						meta := MetricMetadata{
							MetricType:             metricTypeSum,
							ServiceName:            svcName,
							MetricName:             metric.GetName(),
							MetricDescription:      metric.GetDescription(),
							MetricUnit:             metric.GetUnit(),
							ResourceAttributes:     resAttrs,
							ResourceSchemaUrl:      resSchemaUrl,
							ScopeName:              scope.GetName(),
							ScopeVersion:           scope.GetVersion(),
							ScopeAttributes:        scopeAttrs,
							ScopeDroppedAttrCount:  scope.GetDroppedAttributesCount(),
							ScopeSchemaUrl:         sm.GetSchemaUrl(),
							Attributes:             kvToMap(dp.GetAttributes()),
							AggregationTemporality: int32(sum.GetAggregationTemporality()),
							IsMonotonic:            sum.GetIsMonotonic(),
						}
						id, err := mapped.recordSeries(seen, meta)
						if err != nil {
							return MappedMetrics{}, err
						}
						mapped.Sums = append(mapped.Sums, SumRow{
							SeriesID:        id,
							NumberDataPoint: numberDataPoint(dp),
						})
					}
				}
			}
		}
	}
	return mapped, nil
}

// recordSeries returns meta's SeriesID, appending its lookup row the first time that ID is seen.
func (mm *MappedMetrics) recordSeries(seen map[uint64]struct{}, meta MetricMetadata) (uint64, error) {
	id, err := meta.SeriesID()
	if err != nil {
		return 0, fmt.Errorf("computing series id for metric %q: %w", meta.MetricName, err)
	}
	if _, ok := seen[id]; !ok {
		seen[id] = struct{}{}
		mm.Series = append(mm.Series, MetricSeriesRow{
			SeriesID:       id,
			MetricMetadata: meta,
		})
	}
	return id, nil
}

// numberDataPoint converts an OTLP NumberDataPoint into the point fields shared by gauge and sum rows.
func numberDataPoint(dp *metricspb.NumberDataPoint) NumberDataPoint {
	return NumberDataPoint{
		StartTimeUnix: nanosToTime(dp.GetStartTimeUnixNano()),
		TimeUnix:      nanosToTime(dp.GetTimeUnixNano()),
		Value:         numberDataPointValue(dp),
		Flags:         dp.GetFlags(),
	}
}
