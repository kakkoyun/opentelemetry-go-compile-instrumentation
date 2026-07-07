# decision(runtime): traces silently off without an endpoint env; metrics dial localhost unconditionally; OTEL_SDK_DISABLED ignored

Labels: `bug` + discussion
Suggested milestone: **v1 blocker in the compat sense** — these are user-observable defaults that freeze at v1; also two spec deviations
Tested on: main @ 73f867f

## What happens

Three inconsistent behaviors in `pkg/runtime`, all reproduced against an instrumented hello-world:

**1. Traces don't work out of the box.** `setupTraceProvider` returns early unless `OTEL_EXPORTER_OTLP_ENDPOINT` or `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` is set (`pkg/runtime/setup.go:137-146`). `OTEL_TRACES_EXPORTER=console` alone produces zero spans, even though `pkg/runtime/otel_setup.go:26` documents `console` as a valid value and the HTTP hooks log "instrumentation initialized". With the endpoint set, the same binary emits correct client+server spans. The OTel spec's default-endpoint behavior (assume `http://localhost:4318` when unset) is not applied for traces.

**2. Metrics phone home by default.** The meter provider is set up unconditionally via autoexport, so every otelc binary — including one with zero matched instrumentations — exports runtime metrics to `localhost:4318` on the standard interval. Observed directly: 4 connections to a listener on :4318 from an 8-second hello-world run with a shortened export interval. Trace and metric defaults therefore point in opposite directions: traces need explicit opt-in, metrics are opt-out (and there's no documented opt-out besides `OTEL_METRICS_EXPORTER=none`).

**3. `OTEL_SDK_DISABLED=true` does nothing.** The SDK, runtime metrics, and all hooks initialize regardless. The spec defines this variable as the kill switch for the whole SDK.

## Repro

```sh
OTEL_TRACES_EXPORTER=console ./app                       # no spans
OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4318 \
OTEL_TRACES_EXPORTER=console ./app                       # spans appear
OTEL_SDK_DISABLED=true ./app                             # SDK initializes anyway
# metrics dial: listen on :4318, run with OTEL_METRIC_EXPORT_INTERVAL=2000
```

## Why this needs deciding before v1

Whichever way these defaults ship, changing them afterward alters the observable behavior of every instrumented binary — that's the definition of a break under the project's own v1 bar. Right now the defaults are also internally inconsistent, so some change is inevitable; better before the freeze than after.

## Suggested direction

1. Honor `OTEL_SDK_DISABLED` in `SetupOTelSDK` (skip everything, including hook initialization where feasible).
2. Make traces and metrics symmetric. Follow the spec: exporters default to `otlp` with the spec's default endpoint for both signals, and `OTEL_TRACES_EXPORTER`/`OTEL_METRICS_EXPORTER` select or disable pipelines without requiring the endpoint variable. If the project prefers quiet-by-default binaries instead, apply that to both signals — but pick one and document it.
3. Define behavior when the app configures its own SDK (currently the injected `init()` unconditionally overwrites global providers at `pkg/runtime/setup.go:170,193` — first-wins, last-wins, or detect-and-defer needs to be a documented choice), and document the duplicate-span risk for apps already using otelhttp middleware.

Evidence: `a5-summary.txt`, `a5-otlp-listener3.log`.
