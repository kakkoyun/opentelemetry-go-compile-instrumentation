# bug(tool): air-gapped builds (GOPROXY=off) fail with an unactionable wall of module-lookup errors

Labels: `bug`, `effort:low` (for the error UX; full offline support is larger)
Suggested milestone: error UX before v1; offline/vendored support can follow
Tested on: main @ 73f867f
Status: a related but distinct fix (vendoring) is in progress upstream — see below

## What happens

In an environment where the module cache contains exactly the app's own dependencies and `GOPROXY=off` (standard for enterprise/air-gapped CI), a plain `go build` succeeds but `otelc go build` fails during setup with dozens of lines like:

```
go: example.com/app8 imports
        go.opentelemetry.io/otelc/instrumentation/net/http/client imports
        go.opentelemetry.io/otel/codes: module lookup disabled by GOPROXY=off
```

The cause is structural: the injected hook modules need ~45 third-party modules (otel SDK, exporters, grpc, prometheus) that the user's app never depended on, so they are not in the cache, and `go mod tidy` inside `syncDeps` has to fetch them. Nothing in the output tells the user which tool injected these imports or what to do about it.

Note the flip side that makes this easy to miss in testing: on a developer machine whose cache is already warm from previous otelc builds, `GOPROXY=off` builds succeed.

## Suggested fix

- Pre-flight check in setup: before running tidy, verify the required modules are resolvable (cache hit or reachable proxy) and fail with one actionable message ("otelc needs N additional modules that are not in the module cache; run `otelc go build` once with network access, or mirror the following modules: ...", ideally with a machine-readable list).
- Longer term this is another argument for publishing the instrumentation modules (removes the pseudo-version/replace hack) and for vendoring support, so a vendored app can build offline after one `go mod vendor` that includes otelc's additions.

## Status update

[PR #616](https://github.com/open-telemetry/opentelemetry-go-compile-instrumentation/pull/616) ("support projects using vendoring") is open upstream and fixes [#195](https://github.com/open-telemetry/opentelemetry-go-compile-instrumentation/issues/195): when it detects a `vendor/` directory it builds with `-mod=mod` so `go` ignores the (out of sync) vendor tree instead of failing with "inconsistent vendoring". By its own notes it does not yet handle `go work vendor`. This addresses a related but narrower problem than the one here — a vendored project builds even though its vendor tree lacks otelc's injected modules — and does not add the pre-flight check or actionable error message this finding asks for; a plain `GOPROXY=off` build with no vendor directory still fails with the unactionable wall of errors above. Not yet merged as of this audit.

Evidence: [a6-goproxy-off-fresh.log](../a6-goproxy-off-fresh.log), [a6-summary.txt](../a6-summary.txt).
