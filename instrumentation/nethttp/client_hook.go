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

var clientInstrumenter = BuildClientInstrumenter()

// clientContextKey is used to store the instrumentation context in the request context
type clientContextKey struct{}

// clientInstrumentationContext holds the context needed for ending instrumentation
type clientInstrumentationContext struct {
	startTime time.Time
	request   ClientRequest
	ctx       context.Context
}

func BeforeClientDo(ictx inst.HookContext, client interface{}) {
	// Defensive: if anything fails in instrumentation, fail silently and continue with the request
	defer func() {
		if rec := recover(); rec != nil {
			// Log instrumentation panic but don't propagate - let the request continue
			getLogger().Error("instrumentation panic in BeforeClientDo", "panic", rec)
		}
	}()

	// Get request parameter from hook context
	req, ok := ictx.GetParam(1).(*http.Request)
	if !ok || req == nil {
		return
	}

	// Get the parent context from the request
	parentCtx := req.Context()
	if parentCtx == nil {
		parentCtx = context.Background()
	}

	// Create client request wrapper
	clientReq := ClientRequest{Request: req}

	// Start instrumentation (creates span and injects trace context into headers)
	startTime := time.Now()
	ctx := clientInstrumenter.Start(parentCtx, clientReq)

	// Store instrumentation context for use in AfterClientDo
	instrCtx := &clientInstrumentationContext{
		startTime: startTime,
		request:   clientReq,
		ctx:       ctx,
	}
	ctx = context.WithValue(ctx, clientContextKey{}, instrCtx)

	// Update the request with the new context containing trace information
	// This ensures the trace context is propagated through the request
	newReq := req.WithContext(ctx)
	ictx.SetParam(1, newReq)
}

func AfterClientDo(ictx inst.HookContext, client interface{}) {
	// Defensive: if anything fails in instrumentation, fail silently
	defer func() {
		if rec := recover(); rec != nil {
			// Log instrumentation panic but don't propagate
			getLogger().Error("instrumentation panic in AfterClientDo", "panic", rec)
		}
	}()

	// Get parameters from return values
	req, ok := ictx.GetParam(1).(*http.Request)
	if !ok || req == nil {
		return
	}

	// Get return values (resp and err)
	var resp *http.Response
	var err error

	if ictx.GetReturnValCount() >= 1 {
		if r, ok := ictx.GetReturnVal(0).(*http.Response); ok {
			resp = r
		}
	}
	if ictx.GetReturnValCount() >= 2 {
		if e, ok := ictx.GetReturnVal(1).(error); ok {
			err = e
		}
	}

	// Retrieve instrumentation context from request context
	ctx := req.Context()
	if ctx == nil {
		return
	}

	instrCtxVal := ctx.Value(clientContextKey{})
	if instrCtxVal == nil {
		return
	}

	instrCtx, ok := instrCtxVal.(*clientInstrumentationContext)
	if !ok {
		return
	}

	// Create client response wrapper
	clientResp := ClientResponse{
		Response: resp,
		Err:      err,
	}

	// Create invocation for ending instrumentation
	invocation := instrumenter.Invocation[ClientRequest, ClientResponse]{
		Request:        instrCtx.request,
		Response:       clientResp,
		Err:            err,
		StartTimeStamp: instrCtx.startTime,
		EndTimeStamp:   time.Now(),
	}

	// End instrumentation (closes span and records metrics)
	clientInstrumenter.End(instrCtx.ctx, invocation)
}
