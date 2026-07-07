## Description

Honors `OTEL_SDK_DISABLED` (case-insensitive `"true"`, per spec) in `SetupOTelSDK` — nothing is installed when set. Removes the OTLP-endpoint-env gate from trace-provider setup so `OTEL_TRACES_EXPORTER` alone decides (otlp default / console / none), exactly like the meter provider already behaved, and adds the symmetric logs pipeline via autoexport. When a signal's exporter resolves to `none`, no provider is installed at all instead of running an inert batch processor.

## Motivation

Three reproduced inconsistencies in every instrumented binary:

1. `OTEL_SDK_DISABLED=true` was ignored — the spec's kill switch did nothing.
2. Traces were silently off unless `OTEL_EXPORTER_OTLP_ENDPOINT`/`..._TRACES_ENDPOINT` was set: `OTEL_TRACES_EXPORTER=console` produced zero spans while the docs in `otel_setup.go` listed console as valid. First-contact users got no traces and no error.
3. Metrics exported unconditionally (dialing localhost:4318 by default) while traces required opt-in env — the two signals pointed in opposite directions.

## Behavior change to review (flagged deliberately)

With no env vars set, binaries now attempt OTLP export for traces and logs to the spec-default endpoint, matching what metrics always did. That is the spec behavior and makes the signals symmetric, but it is observable: anyone relying on traces-off-by-default sees new connection attempts to localhost:4318. The alternative (quiet-by-default for all signals) is a one-line change per signal; the SIG should pick one before v1 freezes the default. The logs pipeline is new plumbing and currently inert (nothing bridges into it yet) — happy to split it out if preferred.

## Verification

- New unit tests: disabled-flag table (true/TRUE/True vs ""/false/1), console-without-endpoint, none-exporter skip for all three signals. `pkg/runtime` and `pkg` module tests pass; tool tests unaffected.
- End-to-end on an instrumented binary: `OTEL_SDK_DISABLED=true` → no providers, explicit skip log; `OTEL_TRACES_EXPORTER=console` with no endpoint env → spans printed; zero-env run shows the default OTLP attempt (connection refused to localhost:4318, same as metrics).

---

## Checklist

- [x] PR title follows conventional commits format
- [x] Code formatted (gofmt)
- [ ] Linters pass: `make lint` (please run in CI)
- [x] Tests pass (pkg/runtime, pkg, tool)
- [x] Tests added for new functionality
- [x] Tests follow testing guidelines
- [x] Documentation updated (env-var doc comment in otel_setup.go)
- [ ] OpenTelemetry Registry updated (n/a)
