# bug(tool): `-cover` builds crash; PGO builds succeed with all instrumentation silently dropped

Labels: `bug`
Suggested milestone: PGO half before v1 (silent-loss class); `-cover` alongside the go-test fix (same crash)
Tested on: main @ 73f867f
Status: fixes for both halves up for review on fork draft PRs — [kakkoyun#13](https://github.com/kakkoyun/opentelemetry-go-compile-instrumentation/pull/13) (`-cover`) and [kakkoyun#14](https://github.com/kakkoyun/opentelemetry-go-compile-instrumentation/pull/14) (PGO); neither merged upstream (see below)

## What happens

**`otelc go build -cover`** fails on any app:

```
failed to write to file $WORK/b001/otelc.init_otelsdk.go:
  format.Node internal error (6:1: expected 'IDENT', found 'import')
```

Same crash signature as the synthetic test-main failure in the [`otelc go test` finding](06-go-test-broken.md) — the file-rule writer chokes when the compile unit has been rewritten by the toolchain (cover instrumentation) before otelc sees it.

**PGO** is worse because it's silent. With a `default.pgo` in the main package (increasingly common — teams commit it), `otelc go build` exits 0 and produces a binary with **zero** trampoline symbols. PGO-profile compiles are deliberately skipped by the compile-command detection (`tool/util/go.go:57-60`), so the entire build is passed through uninstrumented. The user gets no telemetry and no warning.

## Repro

```sh
otelc go build -cover -o app .     # format.Node internal error

# PGO
go run genprofile.go               # anything that writes a default.pgo
otelc go build -o app .            # exit 0
go tool nm app | grep -c OtelBeforeTrampoline   # 0
```

## Suggested fix

- Fix the file-rule writer crash (shared root cause with go test; one fix should cover both).
- For PGO: the skip at `tool/util/go.go:57-60` presumably exists because the preprofile pass confused command detection. Detect the PGO compile flags properly instead of skipping instrumentation — or, at minimum, fail the build (or loudly warn) when a PGO profile is present, so instrumentation never silently disappears.
- Add `-cover`, PGO, and `-race` app builds to the integration matrix (a `-race` build exercises the GLS runtime fields under the race detector, which is untested today).

## Status update

Both halves now have complete drafted fixes on the fork, verified end to end:

- **`-cover`**: the investigation resolved into fork draft PR [kakkoyun#13](https://github.com/kakkoyun/opentelemetry-go-compile-instrumentation/pull/13) (branch `v1-readiness/cover-fingerprint`), stacked on the [go-test fix](06-go-test-broken.md) ([kakkoyun#12](https://github.com/kakkoyun/opentelemetry-go-compile-instrumentation/pull/12)) and the [cache-identity fix](01-build-cache-staleness.md) ([kakkoyun#9](https://github.com/kakkoyun/opentelemetry-go-compile-instrumentation/pull/9)). The writer crash was the first layer; the distinct second problem was a link-time `fingerprint mismatch` caused by forwarding `-cover` to the nested `go list -export` resolve (default coverage selection instruments only main-module and command-line packages, so the nested resolve produced coverage-instrumented archives of packages the outer build compiles uncovered). The PR stops forwarding `-cover`; `otelc go build -cover` then succeeds with instrumentation present and working coverage output.
- **PGO**: fixed by fork draft PR [kakkoyun#14](https://github.com/kakkoyun/opentelemetry-go-compile-instrumentation/pull/14) (branch `v1-readiness/pgo-not-silent`): removes the obsolete `-pgoprofile` skip and makes the nested import resolution profile-aware (implicit `default.pgo` converted to an explicit `-pgo=<abspath>` for the nested resolve).

Neither is merged upstream; treat both halves as live until they land.

Evidence: [a7-cover.log](../a7-cover.log), [a7-pgo.log](../a7-pgo.log), [a7-summary.txt](../a7-summary.txt).
