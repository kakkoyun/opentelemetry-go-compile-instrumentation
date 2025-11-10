// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package nethttp

import (
	"log/slog"

	"github.com/open-telemetry/opentelemetry-go-compile-instrumentation/pkg/otelsetup"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

const (
	instrumentationName    = "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	instrumentationVersion = "0.1.0"
)

func init() {
	// Initialize OpenTelemetry using shared setup
	otelsetup.Initialize(otelsetup.Config{
		ServiceName:            "net-http-instrumentation",
		ServiceVersion:         instrumentationVersion,
		InstrumentationName:    instrumentationName,
		InstrumentationVersion: instrumentationVersion,
	})
}

// getLogger returns the package logger
func getLogger() *slog.Logger {
	return otelsetup.GetLogger()
}

// getMeterProvider returns the meter provider
func getMeterProvider() *sdkmetric.MeterProvider {
	return otelsetup.GetMeterProvider()
}
