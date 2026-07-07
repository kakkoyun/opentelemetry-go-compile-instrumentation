# bug(tool): `-cover` builds crash; PGO builds succeed with all instrumentation silently dropped

Labels: `bug`
Suggested milestone: PGO half before v1 (silent-loss class); `-cover` alongside the go-test fix (same crash)
Tested on: main @ 73f867f
Status: `-cover` half under investigation; PGO half open (see below)

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

The `otelc go test` fix (branch `v1-readiness/gotest-runtime-file`, commit `f725ece` — see [finding 06](06-go-test-broken.md)) changes the same file-rule writer code path that crashes on `-cover`, since both hit the writer with a setup-time-unresolved package name. A branch (`v1-readiness/cover-fingerprint`) exists that merges the go-test fix and the [cache-identity fix](01-build-cache-staleness.md) as prerequisites, but carries no independent commit of its own yet — whether the shared crash is fully resolved for `-cover`, or whether cover-rewritten sources raise a distinct problem once the writer no longer crashes (the branch name suggests a remaining question about how cover interacts with the cache/build-ID stamp), is still under investigation as of this audit. The PGO silent-skip half has no fix in progress and remains fully open.

Evidence: [a7-cover.log](../a7-cover.log), [a7-pgo.log](../a7-pgo.log), [a7-summary.txt](../a7-summary.txt).
