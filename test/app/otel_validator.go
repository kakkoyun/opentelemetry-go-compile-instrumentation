// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package app provides testing helpers and utilities for OpenTelemetry instrumentation tests.
//
// This package leverages the official OpenTelemetry testing packages:
//   - go.opentelemetry.io/otel/sdk/trace/tracetest for span testing
//   - go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest for metric testing
//
// Example usage:
//
//	// Create a tracer provider with span recorder
//	provider, recorder := app.CreateTestTracerProvider()
//	otel.SetTracerProvider(provider)
//
//	// Run instrumented code...
//
//	// Get recorded spans
//	spans := recorder.Ended()
//
//	// Validate spans
//	validator := app.NewSpanValidator(t, spans)
//	validator.RequireSpanCount(2)
//	serverSpan := validator.RequireSpanWithKind(trace.SpanKindServer)
//	app.ValidateSpanAttributes(t, serverSpan, map[string]interface{}{
//		"http.request.method": "GET",
//		"http.response.status_code": 200,
//	})
package app

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

// SpanValidator provides helper methods for validating trace spans
type SpanValidator struct {
	t     *testing.T
	spans []sdktrace.ReadOnlySpan
}

// NewSpanValidator creates a new span validator
func NewSpanValidator(t *testing.T, spans []sdktrace.ReadOnlySpan) *SpanValidator {
	t.Helper()
	return &SpanValidator{
		t:     t,
		spans: spans,
	}
}

// RequireSpanCount validates the number of spans
func (v *SpanValidator) RequireSpanCount(expected int) *SpanValidator {
	v.t.Helper()
	require.Len(v.t, v.spans, expected, "expected %d spans, got %d", expected, len(v.spans))
	return v
}

// RequireSpanWithName finds a span with the given name
func (v *SpanValidator) RequireSpanWithName(name string) sdktrace.ReadOnlySpan {
	v.t.Helper()
	for _, span := range v.spans {
		if span.Name() == name {
			return span
		}
	}
	require.Fail(v.t, fmt.Sprintf("span with name '%s' not found", name))
	return nil
}

// RequireSpanWithKind finds a span with the given kind
func (v *SpanValidator) RequireSpanWithKind(kind trace.SpanKind) sdktrace.ReadOnlySpan {
	v.t.Helper()
	for _, span := range v.spans {
		if span.SpanKind() == kind {
			return span
		}
	}
	require.Fail(v.t, fmt.Sprintf("span with kind '%s' not found", kind.String()))
	return nil
}

// ValidateSpanAttributes validates that a span has the expected attributes
func ValidateSpanAttributes(t *testing.T, span sdktrace.ReadOnlySpan, expectedAttrs map[string]any) {
	t.Helper()
	attrs := span.Attributes()
	attrMap := make(map[string]any)
	for _, attr := range attrs {
		attrMap[string(attr.Key)] = attr.Value.AsInterface()
	}

	for key, expectedValue := range expectedAttrs {
		actualValue, exists := attrMap[key]
		require.True(t, exists, "attribute '%s' not found in span", key)
		require.Equal(t, expectedValue, actualValue, "attribute '%s' has unexpected value", key)
	}
}

// ValidateSpanHasAttribute validates that a span has an attribute with the given key
func ValidateSpanHasAttribute(t *testing.T, span sdktrace.ReadOnlySpan, key string) {
	t.Helper()
	attrs := span.Attributes()
	for _, attr := range attrs {
		if string(attr.Key) == key {
			return
		}
	}
	require.Fail(t, fmt.Sprintf("attribute '%s' not found in span", key))
}

// ValidateSpanNameContains validates that the span name contains the expected substring
func ValidateSpanNameContains(t *testing.T, span sdktrace.ReadOnlySpan, substring string) {
	t.Helper()
	require.Contains(t, span.Name(), substring, "span name should contain '%s'", substring)
}

// ValidateSpanKind validates that the span has the expected kind
func ValidateSpanKind(t *testing.T, span sdktrace.ReadOnlySpan, expectedKind trace.SpanKind) {
	t.Helper()
	require.Equal(t, expectedKind, span.SpanKind(), "span kind mismatch")
}

// ValidateSpanStatusOK validates that the span status is OK (Unset or Ok)
func ValidateSpanStatusOK(t *testing.T, span sdktrace.ReadOnlySpan) {
	t.Helper()
	status := span.Status()
	require.True(t, status.Code == codes.Unset || status.Code == codes.Ok,
		"span status should be OK or Unset, got: %v", status.Code)
}

// ValidateSpanParent validates that the span has the expected parent
func ValidateSpanParent(t *testing.T, span sdktrace.ReadOnlySpan, parentSpan sdktrace.ReadOnlySpan) {
	t.Helper()
	require.Equal(
		t,
		parentSpan.SpanContext().TraceID(),
		span.SpanContext().TraceID(),
		"spans should have the same trace ID",
	)
	require.Equal(t, parentSpan.SpanContext().SpanID(), span.Parent().SpanID(), "span should have correct parent")
}

// ValidateTraceContextPropagation validates that trace context is propagated between client and server spans
func ValidateTraceContextPropagation(t *testing.T, clientSpan, serverSpan sdktrace.ReadOnlySpan) {
	t.Helper()
	require.Equal(t, clientSpan.SpanContext().TraceID(), serverSpan.SpanContext().TraceID(),
		"client and server spans should have the same trace ID")
	require.Equal(t, clientSpan.SpanContext().SpanID(), serverSpan.Parent().SpanID(),
		"server span should have client span as parent")
}

// MetricValidator provides helper methods for validating metrics
type MetricValidator struct {
	t       *testing.T
	metrics []metricdata.Metrics
}

// NewMetricValidator creates a new metric validator
func NewMetricValidator(t *testing.T, metrics []metricdata.Metrics) *MetricValidator {
	t.Helper()
	return &MetricValidator{
		t:       t,
		metrics: metrics,
	}
}

// RequireMetricExists validates that a metric with the given name exists
func (v *MetricValidator) RequireMetricExists(name string) metricdata.Metrics {
	v.t.Helper()
	for _, metric := range v.metrics {
		if metric.Name == name {
			return metric
		}
	}
	require.Fail(v.t, fmt.Sprintf("metric '%s' not found", name))
	return metricdata.Metrics{}
}

// RequireMetricWithNamePrefix finds metrics with names starting with the given prefix
func (v *MetricValidator) RequireMetricWithNamePrefix(prefix string) []metricdata.Metrics {
	v.t.Helper()
	var matching []metricdata.Metrics
	for _, metric := range v.metrics {
		if strings.HasPrefix(metric.Name, prefix) {
			matching = append(matching, metric)
		}
	}
	require.NotEmpty(v.t, matching, "no metrics found with prefix '%s'", prefix)
	return matching
}

// ValidateHistogramMetric validates that a histogram metric has data points
func ValidateHistogramMetric(t *testing.T, metric metricdata.Metrics) {
	t.Helper()
	histogram, ok := metric.Data.(metricdata.Histogram[float64])
	if !ok {
		histogram64, ok64 := metric.Data.(metricdata.Histogram[int64])
		require.True(t, ok64, "metric '%s' is not a histogram", metric.Name)
		require.NotEmpty(t, histogram64.DataPoints, "histogram metric '%s' has no data points", metric.Name)
		return
	}
	require.NotEmpty(t, histogram.DataPoints, "histogram metric '%s' has no data points", metric.Name)
}

// CreateTestTracerProvider creates a tracer provider for testing with a span recorder
func CreateTestTracerProvider() (*sdktrace.TracerProvider, *tracetest.SpanRecorder) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(recorder),
	)
	return provider, recorder
}

// CreateTestMeterProvider creates a meter provider for testing with an in-memory reader
func CreateTestMeterProvider() (*metric.MeterProvider, *metric.ManualReader) {
	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(
		metric.WithReader(reader),
	)
	return provider, reader
}

// CollectMetrics collects metrics from a manual reader
func CollectMetrics(t *testing.T, reader *metric.ManualReader) []metricdata.Metrics {
	t.Helper()
	var rm metricdata.ResourceMetrics
	err := reader.Collect(context.Background(), &rm)
	require.NoError(t, err, "failed to collect metrics")

	var metrics []metricdata.Metrics
	for _, sm := range rm.ScopeMetrics {
		metrics = append(metrics, sm.Metrics...)
	}
	return metrics
}

const (
	// spanPollInterval is the interval for polling span recorder status
	spanPollInterval = 100 * time.Millisecond
)

// WaitForSpans waits for the expected number of spans to be exported
func WaitForSpans(t *testing.T, recorder *tracetest.SpanRecorder, expectedCount int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if len(recorder.Ended()) >= expectedCount {
			return
		}
		time.Sleep(spanPollInterval)
	}
	require.Fail(t, fmt.Sprintf("timeout waiting for spans: expected %d, got %d",
		expectedCount, len(recorder.Ended())))
}

// AssertMetricsEqual uses the official metricdatatest package to assert metrics equality
func AssertMetricsEqual(t *testing.T, expected, actual metricdata.Metrics, opts ...metricdatatest.Option) {
	t.Helper()
	require.True(t, metricdatatest.AssertEqual(t, expected, actual, opts...),
		"metrics should be equal")
}

// AssertResourceMetricsEqual uses the official metricdatatest package to assert resource metrics equality
func AssertResourceMetricsEqual(
	t *testing.T,
	expected, actual metricdata.ResourceMetrics,
	opts ...metricdatatest.Option,
) {
	t.Helper()
	require.True(t, metricdatatest.AssertEqual(t, expected, actual, opts...),
		"resource metrics should be equal")
}

// PrintSpanTree prints a tree view of spans for debugging
func PrintSpanTree(t *testing.T, spans []sdktrace.ReadOnlySpan) {
	t.Helper()
	t.Log("Span Tree:")
	for _, span := range spans {
		indent := ""
		if span.Parent().IsValid() {
			indent = "  "
		}
		t.Logf("%s- %s (kind=%s, trace_id=%s, span_id=%s)",
			indent, span.Name(), span.SpanKind(),
			span.SpanContext().TraceID(), span.SpanContext().SpanID())
		for _, attr := range span.Attributes() {
			t.Logf("%s    %s = %v", indent, attr.Key, attr.Value.AsInterface())
		}
	}
}
