# net/http OpenTelemetry Instrumentation

This package provides compile-time instrumentation for Go's standard `net/http` package, automatically adding OpenTelemetry tracing and metrics to HTTP servers and clients.

## Overview

Unlike traditional library-based instrumentation that requires manual code changes, this package uses compile-time hooks to automatically instrument HTTP traffic without modifying application code.

## Architecture

### Server Instrumentation

The server instrumentation hooks into `net/http.serverHandler.ServeHTTP`, the internal handler that processes all HTTP requests in the standard library.

**Hook Points:**

- `BeforeServeHTTP`: Called before the HTTP handler processes the request
- `AfterServeHTTP`: Called after the HTTP handler completes

**Implementation Details:**

1. Extracts trace context from incoming HTTP request headers (W3C Trace Context)
2. Creates a new server span with the extracted parent context
3. Wraps the `http.ResponseWriter` to capture status code and response size
4. Updates the request context with the new span context
5. After handler execution, ends the span with response information

**Files:**

- `server_instrumenter.go` - Server instrumenter implementation
- `server_hook.go` - Hook functions for server-side instrumentation
- `response_writer.go` - Response writer wrapper for capturing response data

### Client Instrumentation

The client instrumentation hooks into `http.Client.Do`, the method used to execute HTTP requests.

**Hook Points:**

- `BeforeClientDo`: Called before the HTTP request is sent
- `AfterClientDo`: Called after the HTTP response is received

**Implementation Details:**

1. Creates a new client span in the request context
2. Injects trace context into outgoing HTTP request headers (W3C Trace Context)
3. Updates the request context to include the new span
4. After receiving response, ends the span with response information and any errors

**Files:**

- `client_instrumenter.go` - Client instrumenter implementation
- `client_hook.go` - Hook functions for client-side instrumentation

### Shared Components

**OpenTelemetry Setup** (`otel_setup.go`):

- Initializes OpenTelemetry SDK with OTLP exporters
- Configures trace and metric providers
- Sets up W3C Trace Context propagator
- Supports configuration via environment variables

## Semantic Conventions

This instrumentation follows OpenTelemetry HTTP semantic conventions v1.27.0.

### Server Span Attributes

| Attribute | Type | Description | Example |
|-----------|------|-------------|---------|
| `http.request.method` | string | HTTP method | `GET`, `POST` |
| `http.response.status_code` | int | HTTP status code | `200`, `404` |
| `http.route` | string | HTTP route pattern | `/greet` |
| `network.protocol.version` | string | HTTP version | `1.1`, `2` |
| `network.peer.address` | string | Client IP address | `192.168.1.100` |
| `user_agent.original` | string | User agent string | `curl/7.68.0` |
| `url.scheme` | string | URL scheme | `http`, `https` |
| `url.path` | string | URL path | `/greet` |
| `url.query` | string | URL query string | `name=world` |

### Client Span Attributes

| Attribute | Type | Description | Example |
|-----------|------|-------------|---------|
| `http.request.method` | string | HTTP method | `GET`, `POST` |
| `http.response.status_code` | int | HTTP status code | `200`, `404` |
| `url.full` | string | Complete URL | `http://example.com/greet` |
| `url.scheme` | string | URL scheme | `http`, `https` |
| `server.address` | string | Server hostname | `example.com` |
| `server.port` | int | Server port | `8080` |
| `network.protocol.version` | string | HTTP version | `1.1`, `2` |

### Metrics

**Server Metrics:**

- `http.server.request.duration` - Histogram of request durations (seconds)
- `http.server.request.body.size` - Request body size (bytes)
- `http.server.response.body.size` - Response body size (bytes)

**Client Metrics:**

- `http.client.request.duration` - Histogram of request durations (seconds)
- `http.client.request.body.size` - Request body size (bytes)
- `http.client.response.body.size` - Response body size (bytes)

## Configuration

All configuration is done via environment variables following OpenTelemetry standards.

### OTLP Exporter Configuration

```bash
# Endpoint for OTLP gRPC exporter
OTEL_EXPORTER_OTLP_ENDPOINT="localhost:4317"

# Alternative: separate endpoints for traces and metrics
OTEL_EXPORTER_OTLP_TRACES_ENDPOINT="localhost:4317"
OTEL_EXPORTER_OTLP_METRICS_ENDPOINT="localhost:4317"

# Protocol (currently only grpc is supported)
OTEL_EXPORTER_OTLP_PROTOCOL="grpc"
```

### Service Configuration

```bash
# Service name (shows up in traces and metrics)
OTEL_SERVICE_NAME="my-http-service"

# Additional resource attributes
OTEL_RESOURCE_ATTRIBUTES="service.namespace=production,service.version=1.0.0,deployment.environment=staging"
```

### Logging Configuration

```bash
# Log level for the instrumentation package
OTEL_LOG_LEVEL="info"  # Options: debug, info, warn, error
```

## Usage

### Building Applications with Instrumentation

The instrumentation is applied automatically at compile time using the `otel` tool:

```bash
# Build with instrumentation
otel go build -o myapp .

# The resulting binary includes automatic HTTP instrumentation
./myapp
```

No code changes are required in your application!

### Example: HTTP Server

```go
package main

import (
    "net/http"
)

func main() {
    http.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("Hello, World!"))
    })

    // Automatically instrumented!
    http.ListenAndServe(":8080", nil)
}
```

When compiled with the `otel` tool, this server will:

- Create a span for each incoming request
- Extract trace context from request headers
- Set span attributes based on the request/response
- Record metrics for request duration and sizes

### Example: HTTP Client

```go
package main

import (
    "context"
    "net/http"
)

func main() {
    client := &http.Client{}

    req, _ := http.NewRequestWithContext(context.Background(), "GET", "http://example.com", nil)

    // Automatically instrumented!
    resp, err := client.Do(req)
    if err != nil {
        panic(err)
    }
    defer resp.Body.Close()
}
```

When compiled with the `otel` tool, this client will:

- Create a span for each outgoing request
- Inject trace context into request headers
- Set span attributes based on the request/response
- Record metrics for request duration and sizes

## Context Propagation

The instrumentation automatically handles W3C Trace Context propagation:

1. **Server**: Extracts `traceparent` and `tracestate` headers from incoming requests
2. **Client**: Injects `traceparent` and `tracestate` headers into outgoing requests

This enables distributed tracing across service boundaries without any manual context management.

## Error Handling

The instrumentation is designed to fail silently to ensure user workloads are never disrupted:

- **Instrumentation Errors**: All errors within the instrumentation code are caught and logged, but never propagated to user code
- **User Code Protection**: If the instrumentation encounters any issues (panics, nil pointers, etc.), it logs the error and allows the HTTP request/response to continue normally
- **Seamless Operation**: The instrumentation never interferes with user panics or errors - those propagate normally
- **Server Errors**: HTTP status codes >= 500 are recorded in spans but don't affect request processing
- **Client Errors**: Network errors and timeouts are recorded in spans but handled by user code as normal
- **No-op Fallback**: If OpenTelemetry SDK initialization fails, the instrumentation operates with no-op providers

## Limitations

1. **Route Patterns**: The server instrumentation uses the actual request path as the route since standard `net/http` doesn't expose route patterns. Consider using a router that provides route information for better cardinality control.

2. **Response Headers**: Response headers are not currently captured as attributes.

3. **Request/Response Bodies**: Body content is not captured for privacy and performance reasons. Only body sizes are recorded.

4. **HTTP/2 Push**: HTTP/2 server push is supported but not specially instrumented.

5. **WebSockets**: WebSocket upgrades are instrumented as regular HTTP requests. The subsequent WebSocket communication is not instrumented.

## Performance

The instrumentation is designed to have minimal overhead:

- **Span Creation**: ~1-2 microseconds per request
- **Context Propagation**: ~500 nanoseconds per request
- **Response Wrapping**: ~100 nanoseconds per write

Batch span processing and periodic metric export further reduce the overhead of telemetry export.

## Dependencies

```go
require (
    go.opentelemetry.io/otel v1.33.0
    go.opentelemetry.io/otel/sdk v1.33.0
    go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.33.0
    go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc v1.33.0
)
```

## Development

### Running Tests

```bash
# Run all HTTP instrumentation tests
go test -v ./test/http_test.go ./test/http_client_test.go

# Run with the otel tool
cd test
../otel go test -v -run TestHttp
```

### Debugging

Enable debug logging to see detailed instrumentation information:

```bash
OTEL_LOG_LEVEL=debug ./myapp
```

This will show:

- OpenTelemetry initialization
- Span creation and completion
- Attribute extraction
- Export operations

## Contributing

Contributions are welcome! Areas for improvement:

1. Better route pattern extraction for frameworks like `gorilla/mux` or `chi`
2. Response header capture
3. Support for HTTP/3
4. WebSocket instrumentation
5. More comprehensive error type detection

## License

This instrumentation package is part of the OpenTelemetry Go compile-time instrumentation project and is licensed under the Apache License 2.0.

## References

- [OpenTelemetry HTTP Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/http/http-spans/)
- [W3C Trace Context](https://www.w3.org/TR/trace-context/)
- [Go net/http Package](https://pkg.go.dev/net/http)
- [Project Documentation](../../../docs/)
