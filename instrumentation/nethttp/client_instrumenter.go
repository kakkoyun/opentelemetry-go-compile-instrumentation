// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package nethttp

import (
	"net/http"
	"strconv"

	instrumenter "github.com/open-telemetry/opentelemetry-go-compile-instrumentation/pkg/inst-api"
	semconvhttp "github.com/open-telemetry/opentelemetry-go-compile-instrumentation/pkg/inst-api-semconv/instrumenter/http"
	"github.com/open-telemetry/opentelemetry-go-compile-instrumentation/pkg/inst-api-semconv/instrumenter/net"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/instrumentation"
)

// ClientRequest wraps the HTTP request for client-side instrumentation
type ClientRequest struct {
	*http.Request
}

// ClientResponse wraps the HTTP response for client-side instrumentation
type ClientResponse struct {
	*http.Response
	Err error
}

// ClientAttrsGetter implements HTTPClientAttrsGetter for extracting HTTP client attributes
type ClientAttrsGetter struct{}

// GetRequestMethod returns the HTTP method
func (g ClientAttrsGetter) GetRequestMethod(req ClientRequest) string {
	return req.Method
}

// GetHTTPRequestHeader returns the HTTP request header values for the given name
func (g ClientAttrsGetter) GetHTTPRequestHeader(req ClientRequest, name string) []string {
	return req.Header.Values(name)
}

// GetHTTPResponseStatusCode returns the HTTP response status code
func (g ClientAttrsGetter) GetHTTPResponseStatusCode(req ClientRequest, resp ClientResponse, err error) int {
	if err != nil || resp.Response == nil {
		return 0
	}
	return resp.StatusCode
}

// GetHTTPResponseHeader returns the HTTP response header values for the given name
func (g ClientAttrsGetter) GetHTTPResponseHeader(req ClientRequest, resp ClientResponse, name string) []string {
	if resp.Response == nil {
		return nil
	}
	return resp.Header.Values(name)
}

// GetErrorType returns the error type based on status code or error
func (g ClientAttrsGetter) GetErrorType(req ClientRequest, resp ClientResponse, err error) string {
	if err != nil {
		return "error"
	}
	if resp.Response == nil {
		return ""
	}
	statusCode := resp.StatusCode
	if statusCode >= 400 && statusCode < 600 {
		return strconv.Itoa(statusCode)
	}
	return ""
}

// ClientNetworkAttrsGetter implements network attribute extraction for client requests
type ClientNetworkAttrsGetter struct{}

// GetNetworkType returns the network type
func (g ClientNetworkAttrsGetter) GetNetworkType(req ClientRequest, resp ClientResponse) string {
	return "ipv4"
}

// GetNetworkTransport returns the network transport protocol
func (g ClientNetworkAttrsGetter) GetNetworkTransport(req ClientRequest, resp ClientResponse) string {
	return "tcp"
}

// GetNetworkProtocolName returns the protocol name
func (g ClientNetworkAttrsGetter) GetNetworkProtocolName(req ClientRequest, resp ClientResponse) string {
	return "http"
}

// GetNetworkProtocolVersion returns the protocol version
func (g ClientNetworkAttrsGetter) GetNetworkProtocolVersion(req ClientRequest, resp ClientResponse) string {
	if resp.Response == nil {
		return ""
	}
	if resp.ProtoMajor == 1 && resp.ProtoMinor == 1 {
		return "1.1"
	} else if resp.ProtoMajor == 2 {
		return "2"
	} else if resp.ProtoMajor == 3 {
		return "3"
	}
	return "1.0"
}

// GetNetworkLocalInetAddress returns the local network address
func (g ClientNetworkAttrsGetter) GetNetworkLocalInetAddress(req ClientRequest, resp ClientResponse) string {
	return ""
}

// GetNetworkLocalPort returns the local port
func (g ClientNetworkAttrsGetter) GetNetworkLocalPort(req ClientRequest, resp ClientResponse) int {
	return 0
}

// GetNetworkPeerInetAddress returns the peer network address
func (g ClientNetworkAttrsGetter) GetNetworkPeerInetAddress(req ClientRequest, resp ClientResponse) string {
	return req.URL.Hostname()
}

// GetNetworkPeerPort returns the peer port
func (g ClientNetworkAttrsGetter) GetNetworkPeerPort(req ClientRequest, resp ClientResponse) int {
	port := req.URL.Port()
	if port == "" {
		if req.URL.Scheme == "https" {
			return 443
		}
		return 80
	}
	// Try to parse the port
	if p, err := strconv.Atoi(port); err == nil {
		return p
	}
	return 0
}

// ClientURLAttrsGetter implements URL attribute extraction for client requests
type ClientURLAttrsGetter struct{}

// GetURLScheme returns the URL scheme
func (g ClientURLAttrsGetter) GetURLScheme(req ClientRequest) string {
	return req.URL.Scheme
}

// GetURLPath returns the URL path
func (g ClientURLAttrsGetter) GetURLPath(req ClientRequest) string {
	return req.URL.Path
}

// GetURLQuery returns the URL query string
func (g ClientURLAttrsGetter) GetURLQuery(req ClientRequest) string {
	return req.URL.RawQuery
}

// GetURLFull returns the full URL
func (g ClientURLAttrsGetter) GetURLFull(req ClientRequest) string {
	return req.URL.String()
}

// BuildClientInstrumenter creates an instrumenter for HTTP client operations
func BuildClientInstrumenter() instrumenter.Instrumenter[ClientRequest, ClientResponse] {
	builder := &instrumenter.Builder[ClientRequest, ClientResponse]{}

	clientGetter := ClientAttrsGetter{}

	// Create span name extractor
	spanNameExtractor := &semconvhttp.HTTPClientSpanNameExtractor[ClientRequest, ClientResponse]{
		Getter: clientGetter,
	}

	// Create HTTP attributes extractor
	httpAttrsExtractor := &semconvhttp.HTTPClientAttrsExtractor[ClientRequest, ClientResponse, ClientAttrsGetter]{
		Base: semconvhttp.HTTPCommonAttrsExtractor[ClientRequest, ClientResponse, ClientAttrsGetter]{
			HTTPGetter: clientGetter,
		},
	}

	// Create network attributes extractor
	networkGetter := ClientNetworkAttrsGetter{}
	networkAttrsExtractor := net.CreateNetworkAttributesExtractor[ClientRequest, ClientResponse](networkGetter)

	// Create URL attributes extractor
	urlGetter := ClientURLAttrsGetter{}
	urlAttrsExtractor := &net.URLAttrsExtractor[ClientRequest, ClientResponse, ClientURLAttrsGetter]{
		Getter: urlGetter,
	}

	// Create HTTP client metrics
	metricsRegistry := semconvhttp.NewMetricsRegistry(getLogger(), getMeterProvider().Meter(instrumentationName))
	clientMetrics, err := metricsRegistry.NewHTTPClientMetric(instrumentationName)
	if err != nil {
		getLogger().Error("failed to create HTTP client metrics", "error", err)
	}

	// Build the instrumenter with propagation support
	base := builder.Init().
		SetSpanNameExtractor(spanNameExtractor).
		SetSpanKindExtractor(&instrumenter.AlwaysClientExtractor[ClientRequest]{}).
		AddAttributesExtractor(httpAttrsExtractor, &networkAttrsExtractor, urlAttrsExtractor).
		SetInstrumentationScope(instrumentation.Scope{
			Name:    instrumentationName,
			Version: instrumentationVersion,
		})

	if clientMetrics != nil {
		base.AddOperationListeners(clientMetrics)
	}

	// Build with propagation to downstream (inject trace context into outgoing request)
	return base.BuildPropagatingToDownstreamInstrumenter(
		func(req ClientRequest) propagation.TextMapCarrier {
			return propagation.HeaderCarrier(req.Header)
		},
		nil, // Use default propagator from otel.GetTextMapPropagator()
	)
}
