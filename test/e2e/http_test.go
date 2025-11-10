// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package test

import (
	"bufio"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-go-compile-instrumentation/test/app"
)

func waitUntilReady(t *testing.T, serverApp *exec.Cmd, outputPipe io.ReadCloser) func() string {
	t.Helper()

	readyChan := make(chan struct{})
	doneChan := make(chan struct{})
	output := strings.Builder{}
	const readyMsg = "server started"
	go func() {
		// Scan will return false when the application exits.
		defer close(doneChan)
		scanner := bufio.NewScanner(outputPipe)
		for scanner.Scan() {
			line := scanner.Text()
			output.WriteString(line + "\n")
			if strings.Contains(line, readyMsg) {
				close(readyChan)
			}
		}
	}()

	select {
	case <-readyChan:
		t.Logf("Server is ready!")
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for server to be ready")
	}

	return func() string {
		// Wait for the server to exit
		serverApp.Wait()
		// Wait for the output goroutine to finish
		<-doneChan
		// Return the complete output
		return output.String()
	}
}

func TestHttp(t *testing.T) {
	serverDir := filepath.Join("..", "..", "demo", "http", "server")
	clientDir := filepath.Join("..", "..", "demo", "http", "client")

	// Build the server and client applications with the instrumentation tool.
	app.Build(t, serverDir, "go", "build", "-a")
	app.Build(t, clientDir, "go", "build", "-a")

	// Start the server and wait for it to be ready.
	serverApp, outputPipe := app.Start(t, serverDir)
	waitUntilDone := waitUntilReady(t, serverApp, outputPipe)

	// Run the client to make requests to the server
	clientOutput := app.Run(t, clientDir, "-count", "3", "-method", "GET")
	t.Logf("Client output:\n%s", clientOutput)

	// Run another client request with POST method
	clientOutputPost := app.Run(t, clientDir, "-count", "2", "-method", "POST")
	t.Logf("Client POST output:\n%s", clientOutputPost)

	// Send shutdown request to the server
	app.Run(t, clientDir, "-shutdown")

	// Wait for the server to exit and return the output.
	output := waitUntilDone()

	// Verify that instrumentation hooks were called
	// Note: The actual telemetry validation would require an OTLP collector
	// For now, we verify the hooks are being executed through log messages

	// Verify server instrumentation was active
	// The output should show OpenTelemetry initialization
	require.Contains(t, output, "OpenTelemetry", "server should initialize OpenTelemetry")

	// Verify requests were processed
	require.Contains(t, output, "received request", "server should have processed requests")

	// Log the full output for debugging
	t.Logf("Server output:\n%s", output)

	// Verify client output shows successful requests
	require.Contains(t, clientOutput, "request successful", "client should have made successful requests")
	require.Contains(t, clientOutputPost, "request successful", "client POST requests should be successful")

	// TODO: Add full telemetry validation with OTLP collector
	// - Verify server spans are created with correct attributes
	// - Verify client spans are created with correct attributes
	// - Verify trace context is propagated from client to server
	// - Verify metrics are recorded (http.server.request.duration, http.client.request.duration)
}
