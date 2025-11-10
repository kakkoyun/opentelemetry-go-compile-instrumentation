// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package nethttp

import (
	"context"
	"net/http"
	"time"
	_ "unsafe"

	"github.com/open-telemetry/opentelemetry-go-compile-instrumentation/pkg/inst"
	instrumenter "github.com/open-telemetry/opentelemetry-go-compile-instrumentation/pkg/inst-api"
)

var serverInstrumenter = BuildServerInstrumenter()

// serverContextKey is used to store the instrumentation context in the request context
type serverContextKey struct{}

// serverInstrumentationContext holds the context needed for ending instrumentation
type serverInstrumentationContext struct {
	startTime time.Time
	request   ServerRequest
	writer    *responseWriter
}

func BeforeServeHTTP(ictx inst.HookContext, sh interface{}) {
	// Defensive: if anything fails in instrumentation, fail silently and continue serving the request
	defer func() {
		if rec := recover(); rec != nil {
			// Log instrumentation panic but don't propagate - let the request continue
			getLogger().Error("instrumentation panic in BeforeServeHTTP", "panic", rec)
		}
	}()

	// Get parameters from hook context
	w, ok := ictx.GetParam(1).(http.ResponseWriter)
	if !ok || w == nil {
		return
	}
	r, ok := ictx.GetParam(2).(*http.Request)
	if !ok || r == nil {
		return
	}

	// Wrap the response writer to capture status code and bytes written
	wrappedWriter := newResponseWriter(w)

	// Create server request wrapper
	serverReq := ServerRequest{Request: r}

	// Extract parent context from the request
	parentCtx := r.Context()
	if parentCtx == nil {
		parentCtx = context.Background()
	}

	// Start instrumentation (extracts trace context from headers and creates span)
	startTime := time.Now()
	ctx := serverInstrumenter.Start(parentCtx, serverReq)

	// Store instrumentation context for later use in ending the span
	instrCtx := &serverInstrumentationContext{
		startTime: startTime,
		request:   serverReq,
		writer:    wrappedWriter,
	}
	ctx = context.WithValue(ctx, serverContextKey{}, instrCtx)

	// Update the request with the new context containing trace information
	newReq := r.WithContext(ctx)
	ictx.SetParam(2, newReq)

	// Replace the response writer with our wrapped version
	ictx.SetParam(1, wrappedWriter)
}

func AfterServeHTTP(ictx inst.HookContext, sh interface{}) {
	// Defensive: if anything fails in instrumentation, fail silently
	defer func() {
		if rec := recover(); rec != nil {
			// Log instrumentation panic but don't propagate
			getLogger().Error("instrumentation panic in AfterServeHTTP", "panic", rec)
		}
	}()

	// Get parameters from hook context
	r, ok := ictx.GetParam(2).(*http.Request)
	if !ok || r == nil {
		return
	}

	// Retrieve instrumentation context from request context
	ctx := r.Context()
	if ctx == nil {
		return
	}

	instrCtxVal := ctx.Value(serverContextKey{})
	if instrCtxVal == nil {
		return
	}

	instrCtx, ok := instrCtxVal.(*serverInstrumentationContext)
	if !ok {
		return
	}

	// End instrumentation with response information
	endServerInstrumentation(ctx, instrCtx)
}

// endServerInstrumentation ends the server instrumentation span
func endServerInstrumentation(ctx context.Context, instrCtx *serverInstrumentationContext) {
	if instrCtx == nil {
		return
	}

	// Create server response with captured information
	serverResp := ServerResponse{
		StatusCode:   instrCtx.writer.StatusCode(),
		BytesWritten: instrCtx.writer.BytesWritten(),
	}

	// Create invocation for ending instrumentation
	invocation := instrumenter.Invocation[ServerRequest, ServerResponse]{
		Request:        instrCtx.request,
		Response:       serverResp,
		StartTimeStamp: instrCtx.startTime,
		EndTimeStamp:   time.Now(),
	}

	// End instrumentation (closes span and records metrics)
	serverInstrumenter.End(ctx, invocation)
}
