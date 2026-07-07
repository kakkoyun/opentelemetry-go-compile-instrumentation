# otelc v1 readiness review — findings, reproductions, and benchmarks vs orchestrion

Scope: pre-v1 audit of open-telemetry/opentelemetry-go-compile-instrumentation (main @ 73f867f, 2026-07-07), cross-checked against orchestrion v1.11.0's capabilities and dd-trace-go v2.9.1. Every claim below was reproduced with runnable scripts; logs live alongside this report. "Blocker" is used strictly for decisions that could only be corrected later by breaking users.

## Bottom line

The instrumentation engine is in good shape: trampoline generation, the seven rule types, golden tests, multi-OS CI, and the version-matrix/latest-lib regression harnesses are real strengths, and the benchmark numbers are already competitive with orchestrion (warm builds slightly faster, runtime overhead at parity). The risk is not the engine — it's the orchestration shell around it (cache identity, go.mod mutation, process lifecycle) and a handful of defaults that become contracts at v1. Four items need a decision or fix before the tag because they are compat-irreversible; seven are serious correctness bugs that can technically be fixed in v1.x without breaking anyone, but three of them (cache staleness, go test, PGO) fail silently, which argues for fixing them before first impressions form.

## Compat-critical: decide before the tag (true v1 blockers)

**1. Rule schema freeze vs implementation drift.** docs/rules.md promises `where`-level combinators "at any position"; the loader rejects them (`rule "combinator_rule" has no recognised selector`; underlying: field-presence type inference #546 + `filter.go:185-199` "not yet supported"). `has_directive` similarly documented-but-rejected. #164 marks OP4.1–4.3 done — true only for `where.file`. Freezing the schema in this state guarantees breaking either docs-compliant users or the schema later. Implement or de-document before v1. → draft issue 10

**2. Distribution/config model.** The documented onboarding (`otel.instrumentation.go` importing `go.opentelemetry.io/otelc/instrumentation/...`) cannot work outside this repo: the modules are unpublished (every build log shows `v0.0.0-00010101000000-000000000000` pseudo-versions and the sync.go:158 replace hack), the 61 nested modules are never tagged, and tool files resolve imports before replaces exist. The hook-import restriction plus the embedded bundle also closes the door on third-party instrumentation packages — the orchestrion capability dd-trace-go's migration depends on. #567/#585 are correctly on the roadmap; the point for v1 is that HookContext, the tool-file convention, and module paths freeze at the tag, so the extension path must be designed (not necessarily implemented) before it. → feeds #567/#585 + suggested ADR

**3. Telemetry defaults.** Reproduced: traces are silently OFF unless `OTEL_EXPORTER_OTLP_ENDPOINT`/`..._TRACES_ENDPOINT` is set (`OTEL_TRACES_EXPORTER=console` alone emits nothing, contradicting the tool's own docs); metrics export unconditionally (hello-world dials localhost:4318); `OTEL_SDK_DISABLED` is ignored (spec deviation); injected init overwrites the app's own global providers. These are exactly the kind of observable defaults that freeze at v1. → draft issue 08

**4. Dependency policy + minimum Go.** Every build injects ~45 modules (grpc v1.82, prometheus, x/net, full exporter suite) into the user's module graph; tidy silently upgrades user-pinned versions (warning only); the shipped binary's dependency set diverges from the committed go.mod (SBOM/govulncheck blind spot); embedded modules impose Go >= 1.25 on all target apps while Go 1.24 is still supported upstream. Whichever policy v1 ships becomes the contract. → draft issue 03

## Correctness bugs, all reproduced (fixable in v1.x without breaking, but the silent ones deserve pre-v1 fixes)

| # | Finding | Failure mode | Draft |
|---|---|---|---|
| 1 | Build cache never invalidated on rule/tool change; `-V=full` passes through unstamped | **Silent** missing instrumentation; link failures; user-set GOCACHE poisons plain `go build` | 01 |
| 2 | Rules matched pre-tidy, compiled post-tidy | Hard `cannot replace` failures: any user below hook-pinned lib versions (#494 mechanism, repro'd with grpc v1.70) and any go < 1.25 app (toolchain-switch variant) | 02 |
| 3 | `Commit()` never called in the `otelc go build` path | SIGKILL/OOM/CI-cancel bricks the project; `otelc cleanup` makes it worse (deletes replace targets, restores nothing); even retry builds fail on go.sum | 04 |
| 4 | No locking on `.otelc-build`/go.mod window | Two concurrent builds permanently bake 8 replace directives into go.mod | 05 |
| 5 | `otelc go test` broken for every package shape | Library pkgs: package clash with generated `otelc.runtime.go`; main pkgs: vet failure + file-rule `format.Node` crash on synthetic test-main | 06 |
| 6 | `-cover` crashes; PGO **silently** uninstrumented (0 trampolines, exit 0) | Same file-rule crash as go test; PGO skip at `go.go:57-60` | 07 |
| 7 | GOPROXY=off (air-gapped CI) fails with unactionable errors | Structural: hook deps aren't in the user's module cache | 09 |

Also verified along the way: only 4 of 22 bundled rule sets carry any `version:` gate (static audit table in evidence); repo CI never invokes `otelc go test`, which is why finding 5 survived; CI runs a proper OS/arch matrix but a single Go version, while injecting raw code into `runtime.newproc1` — a Go-version matrix plus a gotip canary is cheap insurance.

## Benchmarks vs orchestrion (goal: faster and more solid)

Small net/http app, 4-core Linux container, go1.25.0; otelc @73f867f vs orchestrion v1.11.0 + dd-trace-go v2.9.1 (net/http contrib); noop export pipelines on both sides; min-of-runs reported, full data in evidence.

| Metric | plain | otelc | orchestrion |
|---|---|---|---|
| Cold build | 10.7s | 26.1s (2.4x) | 23.4s (2.2x) |
| Warm rebuild (touch main.go) | 0.09s | **2.3s** | 2.8s |
| Binary size | 8.9MB | 25.2MB | 23.7MB |
| Goroutine spawn | 469 ns/op | 468 ns/op | 461 ns/op |
| HTTP round-trip | 81.5µs | 95.7µs (+17%) | 94.6µs (+16%) |

Read: runtime overhead is at parity (goroutine-creation GLS hook is not measurable — good); warm builds are already slightly faster than orchestrion; cold builds are ~12% slower, and otelc's ~2.3s fixed warm-build cost is dominated by the per-build setup phase (`go build -a -x -n` dry run + bundle extract + `go mod tidy`) — the clearest optimization target for the "faster than orchestrion" goal. Binary size carries ~1.5MB over orchestrion from always linking every exporter via autoexport (also flagged in draft 03). Caveats: shared container, single small app; the numbers are directional, and the methodology is written up so CI can adopt it (#570).

## Refuted / softened during verification

- "CI is single-platform" — false: ubuntu amd64/arm64, macos, windows all covered. The gap is Go versions, not platforms.
- "docs promise an `otelc pin` command" — false: docs describe the tool-file flow, which is implemented; the unpublished-modules problem is what breaks it, not a missing command.
- "GOPROXY=off always fails" — only when the module cache lacks otelc's deps (i.e., real CI); warm dev machines mask it.
- Windows path handling and bundle extraction (zip-slip) were checked and are fine.

## Suggested ADRs (candidates, not yet written)

1. **Build-cache identity**: how otelc participates in Go's action-ID computation (`-V=full` stamping, GOCACHE policy). Fixes finding 1 by design rather than patch.
2. **Dependency-version policy**: what happens when hook requirements exceed user pins (fail vs consent vs skip), version-gating obligations for bundled rules (implements ADR-0004's promise), and the minimum-Go statement.
3. **v1 compatibility surface**: exactly what the tag freezes — HookContext (per ADR-0002), rule schema (post issue-10 resolution), CLI verbs, `OTELC_*` env vars, tool-file convention — and what is explicitly internal (matched.json, state.json, `.otelc-build` layout, embedded bundle format).
4. **SDK injection defaults**: resolves draft 08 as a recorded decision (spec-compliant defaults, kill switch, coexistence with app-configured SDKs).

## Suggested sequencing for #261

1. Land decisions (ADRs 2–4) — these gate the tag itself.
2. Fix the silent-failure trio: cache identity (01), PGO skip (07), and go test (06) — cheap relative to their first-impression cost; 01's fix is well-understood (orchestrion's `-V=full` approach).
3. Fix lifecycle robustness (04, 05) — one shared mechanism (commit-before-mutate + flock) covers both.
4. Everything else (02 hardening, 09 UX, Go-version CI matrix) rides normal releases.
