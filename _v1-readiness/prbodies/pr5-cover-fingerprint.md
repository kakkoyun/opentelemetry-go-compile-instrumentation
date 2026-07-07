## Description

Stops forwarding `-cover` to the nested `go list -export` subprocess that resolves instrumentation-added imports. Unlike `-race`/`-msan`/`-asan`, which instrument every package in a build uniformly, cmd/go's default coverage selection (no `-coverpkg`) instruments only main-module and command-line packages — and in the nested resolve, the otelc-owned dependency being resolved *is* the command-line package. The nested subprocess therefore produced coverage-instrumented archives of packages the outer build compiles uncovered; both landed in the shared GOCACHE and the final link mixed them, failing `checkFingerprint`.

Stacked on the cache-identity and go-test PRs (the nested resolve must already run through toolexec for archives to be comparable at all).

## Motivation

`otelc go build -cover` failed on any app:

```
link: fingerprint mismatch: go.opentelemetry.io/otelc/pkg/runtime has X,
  import from go.opentelemetry.io/otelc/instrumentation/go.opentelemetry.io/otel/init expecting Y
```

Root cause confirmed empirically, not inferred: `go tool nm` on the nested resolve's archive shows `goCover_*` counter symbols that are absent from the outer build's archive of the same package.

## Verification

- Regression test asserting `-cover` never appears in `extractBuildFlags` output while `-race`/`-msan` forwarding is unchanged.
- End-to-end: `otelc go build -cover` now succeeds with instrumentation present (6 trampoline symbols), and coverage genuinely works — `go tool covdata percent` reports statement coverage from a `GOCOVERDIR` run. Plain, `-race`, and `go test` builds unaffected.
- `go test ./tool/...` green; gofmt/vet clean.

---

## Checklist

- [x] PR title follows conventional commits format
- [x] Code formatted (gofmt)
- [ ] Linters pass: `make lint` (please run in CI)
- [x] Tests pass: `go test ./tool/...`
- [x] Tests added for new functionality
- [x] Tests follow testing guidelines
- [ ] Documentation updated (n/a)
- [ ] OpenTelemetry Registry updated (n/a)
