package main

import (
	"fmt"
	"time"

	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
)

// TODO: placeholder until we implement proper SeriesID generation based on metric "identity"
const (
	metricTypeGauge     = "gauge"
	metricTypeSum       = "sum"
	placeholderSeriesID = uint64(0)
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

// TODO: Series currently contains one row per data point. Once SeriesID computation is implemented, identical series should be deduplicated
type MappedMetrics struct {
	Series []MetricSeriesRow
	Gauges []GaugeRow
	Sums   []SumRow
}

// MapMetrics walks the request and emits a MetricSeriesRow together with the matching
// datapoint row for each gauge and sum metric.
func MapMetrics(resourceMetrics []*metricspb.ResourceMetrics) MappedMetrics {
	var mapped MappedMetrics
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
						mapped.Series = append(mapped.Series, MetricSeriesRow{
							SeriesID:       placeholderSeriesID,
							MetricMetadata: meta,
						})
						mapped.Gauges = append(mapped.Gauges, GaugeRow{
							SeriesID:        placeholderSeriesID,
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
						mapped.Series = append(mapped.Series, MetricSeriesRow{
							SeriesID:       placeholderSeriesID,
							MetricMetadata: meta,
						})
						mapped.Sums = append(mapped.Sums, SumRow{
							SeriesID:        placeholderSeriesID,
							NumberDataPoint: numberDataPoint(dp),
						})
					}
				}
			}
		}
	}
	return mapped
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
