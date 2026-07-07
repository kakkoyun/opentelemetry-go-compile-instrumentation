# bug(tool): build cache is not invalidated when rules or the otelc binary change; shared GOCACHE poisons plain builds

Labels: `bug`
Suggested milestone: fix before v1 (correctness bug in a shipped feature; not a compat break to fix)
Tested on: main @ 73f867f, go1.25.0, linux/amd64

## What happens

`otelc toolexec` only intercepts `compile` and `link` invocations. The `compile -V=full` probe that `cmd/go` uses to compute tool IDs passes through unmodified (`tool/internal/instrument/toolexec.go:355-392`; the detection in `tool/util/go.go:50` requires `-o`/`-p`/`-buildid`, which `-V=full` doesn't have). As a result, nothing about otelc — its version, or the active rule set — is part of the action ID of any package. The GOCACHE under `.otelc-build/gocache` persists across builds (`tool/internal/setup/setup.go:359-377`), and a user-set GOCACHE is used as-is.

Three consequences, all reproduced:

**1. Silent loss of instrumentation.** Build once with a rule set that doesn't cover net/http, then build again with the default rules: the second build reuses the cached uninstrumented `net/http` archive. The binary reports SDK startup but has no HTTP hooks and no trampoline symbols. No warning, no error.

**2. Hard link failures after removing rules.** The reverse direction fails loudly instead of silently:

```
net/http.OtelBeforeTrampoline_RoundTrip3038199408: relocation target
  go.opentelemetry.io/otelc/instrumentation/net/http/client.BeforeRoundTrip not defined
```

**3. A user-set GOCACHE breaks plain builds.** After one `otelc go build` with `GOCACHE` pointing at a shared cache (a common CI setup), a plain `go build` in the same module fails at link with the same relocation errors — the instrumented stdlib archives are picked up by builds that never asked for instrumentation.

## Repro

```sh
mkdir app && cd app && cat > main.go <<'EOF'
package main

import ("fmt"; "io"; "net/http"; "net/http/httptest")

func main() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "hello")
	}))
	defer srv.Close()
	resp, _ := http.Get(srv.URL)
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	fmt.Printf("%s", b)
}
EOF
go mod init example.com/app

# variant 1: silent loss
touch empty-rules.yaml
OTELC_RULES=$PWD/empty-rules.yaml otelc go build -o app-v1 .   # cold cache, no rules
otelc go build -o app-v2 .                                     # default rules, warm cache
go tool nm app-v2 | grep -c OtelBeforeTrampoline               # 0 — should be > 0
./app-v2   # no "HTTP client/server instrumentation initialized" log lines

# variant 3: shared GOCACHE
GOCACHE=$PWD/shared otelc go build -o app-inst .
GOCACHE=$PWD/shared go build -o app-plain .   # fails: relocation target ... not defined
```

## Why

`cmd/go` computes each package's action ID from its sources, its dependencies' build IDs, and the tool IDs — before toolexec runs. otelc's source rewriting happens inside the compile invocation, invisible to the cache key. If the action ID matches, `compile` is never invoked and toolexec never gets a chance to instrument (or to skip instrumenting).

Orchestrion hit the same problem and solves it by intercepting `-V=full` and appending its own version plus a checksum of the active aspects to the output, e.g. `compile version go1.22.5:orchestrion@v0.7.2...;aspects=sha512:...`. That makes rule changes and tool upgrades naturally invalidate the cache, and keeps instrumented and plain artifacts distinct even in a shared GOCACHE.

## Suggested fix

- In `Toolexec`, detect `-V=full` invocations, run the underlying tool, and append `otelc@<version>;rules=<hash>` to its stdout before returning. The rules hash can be computed once during setup (over the serialized matched rule set) and passed to toolexec via env, next to the existing `matched.json` plumbing. `util.Version` is already stamped via ldflags.
- Add a regression test: build, swap `OTELC_RULES`, rebuild with a warm cache, assert the binary's trampoline symbols match the new rules.
- Until the stamp lands, the tool should either refuse a user-set GOCACHE or isolate it the way it isolates its own.

Evidence logs: `a1-dir1-stale-link-failure.log`, `a1-dir2-silent-missing-instrumentation.txt`, `a1-var3-shared-gocache-poisoning.log`.
