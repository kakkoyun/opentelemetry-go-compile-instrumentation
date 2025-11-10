# HTTP Demo

This directory contains a simple HTTP server and client implementation for demonstrating OpenTelemetry compile-time instrumentation.

## Structure

- `server/` - HTTP server implementation
  - `main.go` - Server code with multiple HTTP handlers
- `client/` - HTTP client implementation
  - `main.go` - Client code with support for different HTTP methods

## Prerequisites

- Go 1.23.0 or higher

## Building

### Server

```bash
cd server
go mod tidy
go build -o server .
```

### Client

```bash
cd client
go mod tidy
go build -o client .
```

## Running

### Start the Server

```bash
cd server
./server
# Server will listen on port 8080 by default
# With 10% fault injection rate and up to 500ms random latency
```

#### Server Configuration Options

```bash
# Use a different port
./server -port=8081

# Adjust fault injection rate (0.0 to 1.0, default: 0.1)
./server -fault-rate=0.2

# Adjust maximum random latency in milliseconds (default: 500)
./server -max-latency=1000

# Disable fault injection
./server -no-faults

# Disable artificial latency
./server -no-latency

# Set log level (debug, info, warn, error; default: info)
./server -log-level=debug

# Combine options
./server -port=8081 -fault-rate=0.3 -max-latency=200 -log-level=debug
```

#### Fault Injection Types

The server randomly simulates the following error conditions based on the fault rate:

1. **Internal Server Error (500)** - Simulates server-side errors
2. **Service Unavailable (503)** - Simulates temporary unavailability
3. **Request Timeout (408)** - Simulates slow processing with a 5-second delay

### Run the Client

#### Simple GET Request

```bash
cd client
./client
# Output: Response: Hello world
# Note: May occasionally fail due to server fault injection
```

#### POST Request

```bash
./client -method=POST
# Sends a POST request with JSON payload
```

#### Multiple Requests

```bash
./client -count=5
# Sends 5 consecutive requests
# Useful for testing fault injection and latency patterns
```

#### Custom Options

```bash
# Connect to a different address
./client -addr=http://localhost:8081

# Send a custom name
./client -name="OpenTelemetry"

# Set log level (debug, info, warn, error; default: info)
./client -log-level=debug

# Combine options
./client -addr=http://localhost:8081 -name="Testing" -method=POST -count=3 -log-level=debug

# Send a shutdown request to the server, this will exit the server process gracefully.
./client -shutdown
```

## API Endpoints

The HTTP server provides the following endpoints:

1. **GET /greet** - Returns a simple greeting message
   - Query parameter: `name` (optional, default: "world")
2. **POST /greet** - Accepts a JSON payload with a name and returns a personalized greeting
3. **GET /health** - Health check endpoint (no fault injection or latency)

### Request/Response Formats

**GET /greet** request:

```bash
curl "http://localhost:8080/greet?name=world"
```

**POST /greet** request:

```json
{
  "name": "world"
}
```

**Success Response** (both endpoints):

```json
{
  "message": "Hello world"
}
```

**Error Response** (when faults are injected):

```json
{
  "error": "internal server error"
}
```

## Features

### Structured Logging with slog

Both server and client use Go's structured logging (`log/slog`) with JSON output for better observability:

```json
{
  "time": "2025-11-04T15:42:06.495367+01:00",
  "level": "INFO",
  "msg": "received request",
  "method": "GET",
  "name": "world-1",
  "path": "/greet",
  "status_code": 200,
  "duration_ms": 94
}
```

**Log Levels:**

- **debug**: Detailed information including artificial latency values and request creation
- **info**: Standard operational logs (requests, responses, configuration)
- **warn**: Fault injection events and server errors
- **error**: Request failures and critical errors

**Key Benefits:**

- Machine-readable JSON format
- Structured fields for easy parsing and filtering
- Request duration tracking
- Correlation between client and server logs

### Artificial Latency

The server adds random latency (0 to `max-latency` milliseconds) to simulate network delays and processing time. This is useful for testing timeout handling and performance monitoring.

At debug level, each latency injection is logged:

```json
{"level":"DEBUG","msg":"adding artificial latency","latency_ms":18}
```

### Client: Fault Injection

Random fault injection simulates real-world failure scenarios:

- **10% default rate** (configurable via `-fault-rate`)
- Three types of faults: 500, 503, and 408 errors
- Helps test error handling and retry logic in clients
- Can be disabled with `-no-faults` flag
- All faults are logged with structured context

### Client Resilience

The client:

- Handles error responses gracefully
- Continues processing remaining requests if one fails
- Tracks success/failure counts
- Logs detailed error information with structured fields
- Measures and logs request duration for performance analysis

## OpenTelemetry Instrumentation

This HTTP demo showcases **compile-time instrumentation** using OpenTelemetry. Unlike traditional library-based instrumentation, this approach automatically instruments your code at compile time without requiring manual code changes.

### How It Works

The `otel` tool (from this repository) intercepts the compilation process and automatically injects instrumentation code into both the HTTP server and client:

1. **Server Instrumentation**: Hooks into `net/http.ServeHTTP` to create server spans
2. **Client Instrumentation**: Hooks into `http.Client.Do` to create client spans
3. **Context Propagation**: Automatically propagates W3C Trace Context between client and server

### Building with Instrumentation

To build the applications with instrumentation:

```bash
# Build the otel tool first
cd ../..  # Go to repository root
make build

# Build the server with instrumentation
cd demo/http/server
../../../otel go build -o server .

# Build the client with instrumentation
cd ../client
../../../otel go build -o client .
```

### Telemetry Configuration

Both server and client support OpenTelemetry configuration via environment variables:

```bash
# OTLP Exporter Configuration
export OTEL_EXPORTER_OTLP_ENDPOINT="http://localhost:4317"
export OTEL_EXPORTER_OTLP_PROTOCOL="grpc"

# Service Configuration
export OTEL_SERVICE_NAME="my-http-service"
export OTEL_RESOURCE_ATTRIBUTES="service.namespace=production,service.version=1.0.0"

# Logging Level
export OTEL_LOG_LEVEL="info"  # debug, info, warn, error
```

### Observability Stack with Docker Compose

The easiest way to see the instrumentation in action is using the full observability stack:

```bash
cd ../infrastructure/docker-compose

# Start the full stack (Jaeger, Prometheus, Grafana, OTel Collector)
docker-compose up -d

# Start the HTTP server and client
docker-compose up http-server http-client

# Optional: Run k6 load tests to generate more telemetry
docker-compose --profile load-testing up k6-http
```

Access the observability tools:

- **Jaeger UI**: <http://localhost:16686> - View distributed traces
- **Grafana**: <http://localhost:3000> - View dashboards and metrics
- **Prometheus**: <http://localhost:9090> - Query metrics directly

### Telemetry Data Collected

#### Traces (Spans)

**Server Spans:**

- Span name: `HTTP {method}` or `HTTP {method} {route}`
- Span kind: `SERVER`
- Attributes:
  - `http.request.method` - HTTP method (GET, POST, etc.)
  - `http.response.status_code` - Response status code
  - `http.route` - Request path
  - `network.protocol.version` - HTTP version (1.1, 2, 3)
  - `network.peer.address` - Client IP address
  - `user_agent.original` - User agent string

**Client Spans:**

- Span name: `HTTP {method}`
- Span kind: `CLIENT`
- Attributes:
  - `http.request.method` - HTTP method
  - `http.response.status_code` - Response status code
  - `url.full` - Complete URL
  - `url.scheme` - URL scheme (http, https)
  - `network.protocol.version` - HTTP version
  - `server.address` - Server hostname

#### Metrics

**Server Metrics:**

- `http.server.request.duration` - Histogram of request durations
- `http.server.request.body.size` - Request body sizes
- `http.server.response.body.size` - Response body sizes

**Client Metrics:**

- `http.client.request.duration` - Histogram of request durations
- `http.client.request.body.size` - Request body sizes
- `http.client.response.body.size` - Response body sizes

### Example: Viewing Traces

1. Start the observability stack and applications:

   ```bash
   docker-compose up -d
   docker-compose up http-server http-client
   ```

2. Open Jaeger UI: <http://localhost:16686>

3. Select Service: `http-client` or `http-server`

4. Click "Find Traces"

5. You'll see traces showing the complete request flow:
   - Client span (CLIENT) → Server span (SERVER)
   - Both spans share the same trace ID
   - Server span is a child of the client span
   - All HTTP attributes are visible

### Example: Querying Metrics

1. Open Prometheus UI: <http://localhost:9090>

2. Try these queries:

   ```promql
   # Request rate
   rate(http_server_request_duration_count[1m])

   # 95th percentile latency
   histogram_quantile(0.95, rate(http_server_request_duration_bucket[5m]))

   # Error rate
   rate(http_server_request_duration_count{http_status_code=~"5.."}[1m])
   ```

3. View in Grafana for better visualization: <http://localhost:3000>

### Troubleshooting

**No telemetry data appearing:**

- Check that the OTLP endpoint is configured correctly
- Verify the OTel Collector is running: `docker-compose ps otel-collector`
- Check application logs for OpenTelemetry initialization messages
- Set `OTEL_LOG_LEVEL=debug` for verbose logging

**Traces not connected:**

- Ensure both client and server are using the same OTel Collector endpoint
- Verify W3C trace context headers are being propagated
- Check for clock skew between services

**High cardinality warnings:**

- The `/greet` endpoint uses dynamic names in URLs which is acceptable for this demo
- In production, use route patterns instead of full paths in span names

### Technical Details

For more information about the instrumentation implementation, see:

- [Instrumentation Package README](../../pkg/instrumentation/nethttp/README.md)
- [Architecture Documentation](../../docs/implementation.md)
- [Hook Configuration](../../tool/data/nethttp.yaml)
