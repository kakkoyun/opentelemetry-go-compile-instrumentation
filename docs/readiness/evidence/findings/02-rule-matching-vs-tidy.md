# bug(tool): rules are matched against pre-tidy module versions; any version drift breaks the build ("cannot replace")

Labels: `bug`
Suggested milestone: fix before v1 — this is the general mechanism behind [#494](https://github.com/open-telemetry/opentelemetry-go-compile-instrumentation/issues/494)
Tested on: main @ 73f867f, go1.25.0, linux/amd64
Status: **fixed upstream** — [PR #655](https://github.com/open-telemetry/opentelemetry-go-compile-instrumentation/pull/655) merged as `238f23d` (2026-07-08); re-verified fixed at `ad45522` (see status update below)

## What happens

`Setup` runs in this order (`tool/internal/setup/setup.go:297→315→349`): discover dependencies and their versions, match rules and record the matched packages' absolute source paths, then `syncDeps` + `go mod tidy`. The tidy step can change module resolution — and when it does, the recorded paths no longer match what the compiler is fed, and the build dies in `writeInstrumented` (`tool/internal/instrument/apply_func.go:329`).

Two reproduced failure modes:

**1. User pins an older version of an instrumented library.** With `google.golang.org/grpc v1.70.0` in go.mod (grpc's hooks require v1.82.0, and the grpc rules carry no `version:` range so they match anything):

```
cannot replace /root/go/pkg/mod/google.golang.org/grpc@v1.70.0/clientconn.go
  with $WORK/b317/clientconn.go during [compile ... -p google.golang.org/grpc
  ... /root/go/pkg/mod/google.golang.org/grpc@v1.82.0/backoff.go ...]
```

Matching recorded `@v1.70.0` paths; tidy bumped grpc to v1.82.0 to satisfy the injected hook module; the compile received `@v1.82.0` sources. This is almost certainly the mechanism behind [#494](https://github.com/open-telemetry/opentelemetry-go-compile-instrumentation/issues/494).

**2. App with `go 1.24.0` in go.mod.** A plain build works. `otelc go build` fails the same way on the `runtime` package: the dependency-discovery pass ran under the host's go1.24.7 toolchain (`/usr/local/go1.24.7/src/runtime/...`), then tidy raised the module's Go requirement to 1.25 (the embedded modules all declare `go 1.25.0`), the real build switched to the downloaded go1.25.0 toolchain, and the recorded stdlib paths went stale:

```
cannot replace /usr/local/go1.24.7/src/runtime/runtime2.go with $WORK/b009/runtime2.go
  during [compile ... /root/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.0.../src/runtime/...]
```

Net effect: any app whose go directive is below 1.25, and any app pinning an instrumented library below what the hook modules require, cannot build with otelc — with an error that gives the user no hint of the cause.

## Repro

```sh
# mode 1
mkdir grpcapp && cd grpcapp && go mod init example.com/grpcapp
cat > main.go <<'EOF'
package main

import "google.golang.org/grpc"

func main() { grpc.NewServer().Stop() }
EOF
go get google.golang.org/grpc@v1.70.0 && go mod tidy
go build -o /dev/null .   # works
otelc go build -o app .   # fails: cannot replace .../grpc@v1.70.0/clientconn.go

# mode 2
mkdir app124 && cd app124 && go mod init example.com/app124 && go mod edit -go=1.24.0
# (any main.go that uses net/http)
go build -o /dev/null .   # works
otelc go build -o app .   # fails: cannot replace /usr/local/go1.24.x/src/runtime/runtime2.go
```

## Why

Rule matching keys `FuncRules` to absolute file paths captured before dependency sync (`tool/internal/rule/base.go:116`), but `syncDeps` + tidy can move any matched package to a different version (or a different toolchain root). The design assumes module resolution is stable across the setup boundary; it is not, precisely because setup itself changes go.mod.

## Suggested fix

Options, roughly in order of robustness:

1. Re-run dependency discovery + matching after `syncDeps`, so recorded paths reflect post-tidy resolution. Costs a second `go list`/dry-run pass; correctness first.
2. Key rules to (import path, module version) instead of absolute paths, and resolve paths at toolexec time from the compile arguments themselves (`match()` already receives the import path; the file list is in the compile args).
3. Independently: ship `version:` ranges on all bundled rules reflecting what each hook module actually supports (see companion issue on version policy), so an unsupported combination is reported as such instead of crashing.

Also worth doing regardless: when `writeInstrumented` can't find the original file, print the likely cause (module version changed between setup and build) instead of the bare "cannot replace" — this will be many users' first contact with the tool.

## Status update

[PR #655](https://github.com/open-telemetry/opentelemetry-go-compile-instrumentation/pull/655) ("add `otelc pin` command") implements option 1 above as a side effect of a larger change: it moves `syncDeps` into the new `Pin` step and re-parses compile commands against the post-tidy dependency graph before matching. Its description states this closes [#494](https://github.com/open-telemetry/opentelemetry-go-compile-instrumentation/issues/494) directly.

**Re-verified 2026-07-09, after #655 merged (`238f23d`), against upstream main `ad45522`:** both repro modes above now **build successfully in the default flow** (no explicit `otelc pin` needed). The grpc-v1.70.0 app with a `go 1.24.7` directive — i.e. both failure modes at once — produced a fully instrumented binary (`otelc go build` exit 0; `go tool nm | grep -c OtelBeforeTrampoline` → 10), with explicit notices during the build (`Bumped dependency google.golang.org/grpc (v1.70.0 -> v1.82.0)`, `Bumped go version (1.24.7 -> 1.25.0)`) and the user's go.mod restored to its original pins afterward. Rerun log: [a2-pin-recheck.log](../a2-pin-recheck.log).

Still open regardless: #655 does not add `version:` ranges (option 3) and the improved "cannot replace" error message (for any residual paths into `writeInstrumented`) was not part of it. The silent-upgrade *policy* question — the bump is now loud, but still automatic — moves to [ADR-0006](../../../adr/0006-dependency-version-policy-for-bundled-instrumentation.md).

Original evidence logs: [a2-grpc-pinned-low.log](../a2-grpc-pinned-low.log), [a2-go124-directive.log](../a2-go124-directive.log), [a2-summary.txt](../a2-summary.txt).
