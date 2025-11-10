// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-telemetry/opentelemetry-go-compile-instrumentation/test/app"
	"github.com/stretchr/testify/require"
)

// TestHttpClient tests the HTTP client instrumentation in isolation
// by using a mock HTTP server without instrumentation
func TestHttpClient(t *testing.T) {
	// Create a mock HTTP server without instrumentation
	requestCount := 0
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		// Log the request for debugging
		t.Logf("Mock server received request: %s %s", r.Method, r.URL.Path)

		// Check for trace context headers (W3C Trace Context)
		traceParent := r.Header.Get("traceparent")
		if traceParent != "" {
			t.Logf("Received traceparent header: %s", traceParent)
		}
		traceState := r.Header.Get("tracestate")
		if traceState != "" {
			t.Logf("Received tracestate header: %s", traceState)
		}

		// Handle different endpoints
		switch r.URL.Path {
		case "/greet":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			response := map[string]string{"message": "Hello from mock server"}
			json.NewEncoder(w).Encode(response)
		case "/error":
			// Simulate an error response
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"})
		case "/shutdown":
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer mockServer.Close()

	t.Logf("Mock server started at: %s", mockServer.URL)

	// Build the client application with instrumentation
	clientDir := filepath.Join("..", "..", "demo", "http", "client")
	app.Build(t, clientDir, "go", "build", "-a")

	// Test 1: Successful GET requests
	t.Run("successful_get_requests", func(t *testing.T) {
		output := app.Run(t, clientDir,
			"-addr", mockServer.URL,
			"-count", "3",
			"-method", "GET",
		)

		t.Logf("Client GET output:\n%s", output)

		// Verify successful requests
		require.Contains(t, output, "request successful",
			"client should have made successful GET requests")
		require.Contains(t, output, "Hello from mock server",
			"client should receive response from mock server")
	})

	// Test 2: Successful POST requests
	t.Run("successful_post_requests", func(t *testing.T) {
		output := app.Run(t, clientDir,
			"-addr", mockServer.URL,
			"-count", "2",
			"-method", "POST",
		)

		t.Logf("Client POST output:\n%s", output)

		// Verify successful POST requests
		require.Contains(t, output, "request successful",
			"client should have made successful POST requests")
	})

	// Test 3: Error handling (server returns 500)
	t.Run("error_handling", func(t *testing.T) {
		output := app.Run(t, clientDir,
			"-addr", mockServer.URL+"/error",
			"-count", "1",
			"-method", "GET",
		)

		t.Logf("Client error handling output:\n%s", output)

		// Client should handle server errors gracefully
		// The output might contain warnings about the error
		require.True(
			t,
			strings.Contains(output, "error") || strings.Contains(output, "500") ||
				strings.Contains(output, "internal server error"),
			"client should handle server errors",
		)
	})

	// Verify that the mock server received requests
	require.Greater(t, requestCount, 0, "mock server should have received requests")
	t.Logf("Mock server received %d total requests", requestCount)

	// TODO: Add validation for:
	// - Client spans are created with correct attributes (http.request.method, url.full, etc.)
	// - Client metrics are recorded (http.client.request.duration)
	// - Trace context is properly injected into outgoing requests (traceparent header)
	// - Error spans are marked with error status when requests fail
}

// TestHttpClientTimeout tests that the client instrumentation handles timeouts correctly
func TestHttpClientTimeout(t *testing.T) {
	// Create a slow mock server that doesn't respond quickly
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// This handler intentionally doesn't respond quickly to test timeout
		select {
		case <-r.Context().Done():
			// Client timeout or cancellation
			t.Log("Request cancelled or timed out")
			return
		}
	}))
	defer mockServer.Close()

	t.Logf("Slow mock server started at: %s", mockServer.URL)

	// Note: The current client has a 10-second timeout by default
	// We would need to modify the client to have a shorter timeout to test this properly
	// For now, this test documents the timeout behavior

	// TODO: Implement timeout test when client timeout is configurable
	t.Skip("Skipping timeout test - requires configurable client timeout")
}

// TestHttpClientConnectionRefused tests that the client instrumentation handles connection errors
func TestHttpClientConnectionRefused(t *testing.T) {
	// Build the client application with instrumentation
	clientDir := filepath.Join("..", "..", "demo", "http", "client")
	app.Build(t, clientDir, "go", "build", "-a")

	// Try to connect to a non-existent server
	// Use a port that is unlikely to be in use
	nonExistentURL := "http://localhost:59999"

	output := app.Run(t, clientDir,
		"-addr", nonExistentURL,
		"-count", "1",
		"-method", "GET",
	)

	t.Logf("Client connection refused output:\n%s", output)

	// The client should log the connection error
	require.True(
		t,
		strings.Contains(output, "error") || strings.Contains(output, "failed") ||
			strings.Contains(output, "connection refused"),
		"client should report connection error",
	)

	// TODO: Validate that the error is properly recorded in the span
	// - Span should have error status
	// - Span should have error attributes
}

// TestHttpClientMultipleMethods tests that the client can handle different HTTP methods
func TestHttpClientMultipleMethods(t *testing.T) {
	methodsSeen := make(map[string]int)

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methodsSeen[r.Method]++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := map[string]string{
			"message": fmt.Sprintf("Received %s request", r.Method),
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	clientDir := filepath.Join("..", "..", "demo", "http", "client")
	app.Build(t, clientDir, "go", "build", "-a")

	// Test GET
	outputGET := app.Run(t, clientDir,
		"-addr", mockServer.URL,
		"-count", "2",
		"-method", "GET",
	)
	require.Contains(t, outputGET, "request successful", "GET requests should succeed")

	// Test POST
	outputPOST := app.Run(t, clientDir,
		"-addr", mockServer.URL,
		"-count", "2",
		"-method", "POST",
	)
	require.Contains(t, outputPOST, "request successful", "POST requests should succeed")

	// Verify both methods were used
	require.Equal(t, 2, methodsSeen["GET"], "should have received 2 GET requests")
	require.Equal(t, 2, methodsSeen["POST"], "should have received 2 POST requests")

	t.Logf("Methods seen: %v", methodsSeen)
}
