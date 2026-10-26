# decision(tool): every build silently adds ~45 dependencies (grpc, prometheus, x/net, ...) and imposes Go >= 1.25 on the target app

Labels: `scope:feat`, discussion
Suggested milestone: **v1 blocker in the compat sense** — these defaults become contracts the moment v1 ships, and tightening them later is a breaking change
Tested on: main @ 73f867f

## What happens

The `init_sdk` file rule (`instrumentation/go.opentelemetry.io/otel/init/otelc.yaml`) targets `main` and matches unconditionally, so `pkg/runtime` and its whole tree are linked into every build. During the build, a stdlib-only hello-world's go.mod temporarily contains, among ~45 entries:

```
go.opentelemetry.io/otel v1.44.0
go.opentelemetry.io/contrib/exporters/autoexport v0.69.0
google.golang.org/grpc v1.82.0
github.com/prometheus/client_golang v1.23.2
golang.org/x/net v0.55.0
go.opentelemetry.io/otel/exporters/otlp/... (8 exporter modules)
```

For an app that already depends on any of these, `go mod tidy` applies MVS and silently upgrades the user's version; `warnVersion` (`tool/internal/setup/sync.go:89-121`) prints a line to stdout and moves on. The built binary embeds the bumped versions while the restored go.mod still shows the old ones — so repo-side tooling (govulncheck, dependabot, SBOM generation) audits a dependency set that is not what ships.

Separately, all embedded modules declare `go 1.25.0`, which means otelc requires every target app to be on go >= 1.25 (apps on 1.24 currently fail with an unrelated-looking error — see the companion "cannot replace" finding). Go 1.24 is still a supported upstream release.

## Why this needs a decision before v1

Whatever v1 does here is the contract:

- If v1 ships "SDK and exporter tree always linked", removing or narrowing that later changes observable behavior (binaries stop exporting) — a break.
- If v1 ships "tidy silently bumps user dependencies", making that an error later breaks existing builds.
- The minimum-Go floor is part of the public compatibility statement and should be chosen deliberately (e.g. "two most recent Go releases", matching what orchestrion promises), not inherited from the embedded modules' go directives.

## Suggested direction

1. Split `pkg/runtime` so the always-injected core doesn't drag grpc/prometheus/x/net into apps that don't use OTLP-over-grpc or the prometheus bridge. autoexport is convenient but pulls every exporter; consider linking only the OTLP-http path by default and gating the rest behind the selected exporter.
2. Turn silent major/minor bumps of the user's existing direct dependencies into an explicit failure with a clear message ("your grpc v1.70.0 is below the v1.82.0 required by otelc's grpc instrumentation; upgrade or disable the grpc integration"), keeping the silent path only for additions that don't touch user-pinned versions.
3. Document the effective dependency delta (or add an `otelc` flag that prints it) so security tooling can account for what actually ships in the binary.
4. Decide and document the minimum-Go policy; if 1.24 support is wanted, the embedded modules' go directives need lowering before v1.

## Status update

This is a policy decision, not a bug with a patch — see [ADR-0006 (draft)](../../../adr/0006-dependency-version-policy-for-bundled-instrumentation.md), which lays out this finding and the mechanism finding ([02](02-rule-matching-vs-tidy.md)) as options with consequences, without picking a winner. [PR #655](https://github.com/open-telemetry/opentelemetry-go-compile-instrumentation/pull/655) reorders dependency sync relative to matching (see finding 02) but does not change the silent-MVS-bump policy or the minimum-Go statement discussed here; both remain open.

Evidence: [a2-gomod-during-build.txt](../a2-gomod-during-build.txt) (full mid-build go.mod of a hello-world), [a2-static-version-audit.txt](../a2-static-version-audit.txt).
