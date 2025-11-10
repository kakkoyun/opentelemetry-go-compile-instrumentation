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

// ServerRequest wraps the HTTP request for server-side instrumentation
type ServerRequest struct {
	*http.Request
}

// ServerResponse wraps the HTTP response for server-side instrumentation
type ServerResponse struct {
	StatusCode   int
	BytesWritten int64
}

// ServerAttrsGetter implements HTTPServerAttrsGetter for extracting HTTP server attributes
type ServerAttrsGetter struct{}

// GetRequestMethod returns the HTTP method
func (g ServerAttrsGetter) GetRequestMethod(req ServerRequest) string {
	return req.Method
}

// GetHTTPRequestHeader returns the HTTP request header values for the given name
func (g ServerAttrsGetter) GetHTTPRequestHeader(req ServerRequest, name string) []string {
	return req.Header.Values(name)
}

// GetHTTPResponseStatusCode returns the HTTP response status code
func (g ServerAttrsGetter) GetHTTPResponseStatusCode(req ServerRequest, resp ServerResponse, err error) int {
	if err != nil {
		return http.StatusInternalServerError
	}
	if resp.StatusCode == 0 {
		return http.StatusOK
	}
	return resp.StatusCode
}

// GetHTTPResponseHeader returns the HTTP response header values for the given name
func (g ServerAttrsGetter) GetHTTPResponseHeader(req ServerRequest, resp ServerResponse, name string) []string {
	// Response headers are not captured in this implementation
	return nil
}

// GetErrorType returns the error type based on status code or error
func (g ServerAttrsGetter) GetErrorType(req ServerRequest, resp ServerResponse, err error) string {
	if err != nil {
		return "error"
	}
	statusCode := g.GetHTTPResponseStatusCode(req, resp, err)
	if statusCode >= 400 && statusCode < 600 {
		return strconv.Itoa(statusCode)
	}
	return ""
}

// GetHTTPRoute returns the HTTP route pattern
func (g ServerAttrsGetter) GetHTTPRoute(req ServerRequest) string {
	// In standard net/http, route information is not available at this level
	// Returns the request path as fallback
	return req.URL.Path
}

// NetworkAttrsGetter implements network attribute extraction for server requests
type NetworkAttrsGetter struct{}

// GetNetworkType returns the network type
func (g NetworkAttrsGetter) GetNetworkType(req ServerRequest, resp ServerResponse) string {
	return "ipv4" // Default to ipv4, could be enhanced to detect actual network type
}

// GetNetworkTransport returns the network transport protocol
func (g NetworkAttrsGetter) GetNetworkTransport(req ServerRequest, resp ServerResponse) string {
	return "tcp"
}

// GetNetworkProtocolName returns the protocol name
func (g NetworkAttrsGetter) GetNetworkProtocolName(req ServerRequest, resp ServerResponse) string {
	return "http"
}

// GetNetworkProtocolVersion returns the protocol version
func (g NetworkAttrsGetter) GetNetworkProtocolVersion(req ServerRequest, resp ServerResponse) string {
	if req.ProtoMajor == 1 && req.ProtoMinor == 1 {
		return "1.1"
	} else if req.ProtoMajor == 2 {
		return "2"
	} else if req.ProtoMajor == 3 {
		return "3"
	}
	return "1.0"
}

// GetNetworkLocalInetAddress returns the local network address
func (g NetworkAttrsGetter) GetNetworkLocalInetAddress(req ServerRequest, resp ServerResponse) string {
	return req.Host
}

// GetNetworkLocalPort returns the local port
func (g NetworkAttrsGetter) GetNetworkLocalPort(req ServerRequest, resp ServerResponse) int {
	// Port extraction would require parsing the Host header
	return 0
}

// GetNetworkPeerInetAddress returns the peer network address
func (g NetworkAttrsGetter) GetNetworkPeerInetAddress(req ServerRequest, resp ServerResponse) string {
	return req.RemoteAddr
}

// GetNetworkPeerPort returns the peer port
func (g NetworkAttrsGetter) GetNetworkPeerPort(req ServerRequest, resp ServerResponse) int {
	// Port extraction would require parsing RemoteAddr
	return 0
}

// URLAttrsGetter implements URL attribute extraction for server requests
type URLAttrsGetter struct{}

// GetURLScheme returns the URL scheme
func (g URLAttrsGetter) GetURLScheme(req ServerRequest) string {
	if req.TLS != nil {
		return "https"
	}
	return "http"
}

// GetURLPath returns the URL path
func (g URLAttrsGetter) GetURLPath(req ServerRequest) string {
	return req.URL.Path
}

// GetURLQuery returns the URL query string
func (g URLAttrsGetter) GetURLQuery(req ServerRequest) string {
	return req.URL.RawQuery
}

// GetURLFull returns the full URL
func (g URLAttrsGetter) GetURLFull(req ServerRequest) string {
	scheme := g.GetURLScheme(req)
	return scheme + "://" + req.Host + req.URL.RequestURI()
}

// BuildServerInstrumenter creates an instrumenter for HTTP server operations
func BuildServerInstrumenter() instrumenter.Instrumenter[ServerRequest, ServerResponse] {
	builder := &instrumenter.Builder[ServerRequest, ServerResponse]{}

	serverGetter := ServerAttrsGetter{}

	// Create span name extractor
	spanNameExtractor := &semconvhttp.HTTPServerSpanNameExtractor[ServerRequest, ServerResponse]{
		Getter: serverGetter,
	}

	// Create HTTP attributes extractor
	httpAttrsExtractor := &semconvhttp.HTTPServerAttrsExtractor[ServerRequest, ServerResponse, ServerAttrsGetter]{
		Base: semconvhttp.HTTPCommonAttrsExtractor[ServerRequest, ServerResponse, ServerAttrsGetter]{
			HTTPGetter: serverGetter,
		},
	}

	// Create network attributes extractor
	networkGetter := NetworkAttrsGetter{}
	networkAttrsExtractor := net.CreateNetworkAttributesExtractor[ServerRequest, ServerResponse](networkGetter)

	// Create URL attributes extractor
	urlGetter := URLAttrsGetter{}
	urlAttrsExtractor := &net.URLAttrsExtractor[ServerRequest, ServerResponse, URLAttrsGetter]{
		Getter: urlGetter,
	}

	// Create HTTP server metrics
	metricsRegistry := semconvhttp.NewMetricsRegistry(getLogger(), getMeterProvider().Meter(instrumentationName))
	serverMetrics, err := metricsRegistry.NewHTTPServerMetric(instrumentationName)
	if err != nil {
		getLogger().Error("failed to create HTTP server metrics", "error", err)
	}

	// Build the instrumenter with propagation support
	base := builder.Init().
		SetSpanNameExtractor(spanNameExtractor).
		SetSpanKindExtractor(&instrumenter.AlwaysServerExtractor[ServerRequest]{}).
		AddAttributesExtractor(httpAttrsExtractor, &networkAttrsExtractor, urlAttrsExtractor).
		SetInstrumentationScope(instrumentation.Scope{
			Name:    instrumentationName,
			Version: instrumentationVersion,
		})

	if serverMetrics != nil {
		base.AddOperationListeners(serverMetrics)
	}

	// Build with propagation from upstream (extract trace context from incoming request)
	return base.BuildPropagatingFromUpstreamInstrumenter(
		func(req ServerRequest) propagation.TextMapCarrier {
			return propagation.HeaderCarrier(req.Header)
		},
		nil, // Use default propagator from otel.GetTextMapPropagator()
	)
}
