package main

import (
	"encoding/json"
	"fmt"

	"github.com/cespare/xxhash/v2"
)

// seriesIdentity is the collection of fields that identify a unique metric series
type seriesIdentity struct {
	MetricType string `json:"metric_type"`
	MetricName string `json:"metric_name"`
	MetricUnit string `json:"metric_unit"`

	ResourceAttributes map[string]string `json:"resource_attributes"`
	ResourceSchemaUrl  string            `json:"resource_schema_url"`

	ScopeName       string            `json:"scope_name"`
	ScopeVersion    string            `json:"scope_version"`
	ScopeAttributes map[string]string `json:"scope_attributes"`
	ScopeSchemaUrl  string            `json:"scope_schema_url"`

	Attributes map[string]string `json:"attributes"`

	AggregationTemporality int32 `json:"aggregation_temporality"`
	IsMonotonic            bool  `json:"is_monotonic"`
}

// SeriesID computes a stable 64-bit identity for the metric series described by m.
func (m MetricMetadata) SeriesID() (uint64, error) {
	identity := seriesIdentity{
		MetricType:             m.MetricType,
		MetricName:             m.MetricName,
		MetricUnit:             m.MetricUnit,
		ResourceAttributes:     emptyIfNil(m.ResourceAttributes),
		ResourceSchemaUrl:      m.ResourceSchemaUrl,
		ScopeName:              m.ScopeName,
		ScopeVersion:           m.ScopeVersion,
		ScopeAttributes:        emptyIfNil(m.ScopeAttributes),
		ScopeSchemaUrl:         m.ScopeSchemaUrl,
		Attributes:             emptyIfNil(m.Attributes),
		AggregationTemporality: m.AggregationTemporality,
		IsMonotonic:            m.IsMonotonic,
	}

	// For deterministic encoding, I used the json.Marshal function here
	// for an actual prod implementation I would use / implement a lower allocation encoder
	encoded, err := json.Marshal(identity)
	if err != nil {
		return 0, fmt.Errorf("marshalling series identity: %w", err)
	}

	return xxhash.Sum64(encoded), nil
}

// emptyIfNil returns m, or a non-nil empty map when m is nil, so that a nil and an
// empty attribute map produce the same series identity.
func emptyIfNil(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return m
}
