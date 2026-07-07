# bug(tool): air-gapped builds (GOPROXY=off) fail with an unactionable wall of module-lookup errors

Labels: `bug`, `effort:low` (for the error UX; full offline support is larger)
Suggested milestone: error UX before v1; offline/vendored support can follow (relates to #195)
Tested on: main @ 73f867f

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
- Longer term this is another argument for publishing the instrumentation modules (removes the pseudo-version/replace hack) and for vendoring support (#195) — a vendored app should be buildable offline after one `go mod vendor` that includes otelc's additions.

Evidence: `a6-goproxy-off-fresh.log`, `a6-summary.txt`.
