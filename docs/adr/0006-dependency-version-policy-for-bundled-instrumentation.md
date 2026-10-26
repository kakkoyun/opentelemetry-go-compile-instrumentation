# 6. Dependency Version Policy for Bundled Instrumentation

Date: 2026-07-07

## Status

Draft / Proposed

## Context

The pre-v1 readiness review (`docs/readiness/README.md`) reproduced two related problems in how bundled instrumentation interacts with a target application's own dependency versions:

**Rules are matched before the dependency graph is final.** `Setup` matches rules and records the matched packages' absolute source paths, then runs `syncDeps` and `go mod tidy` (`tool/internal/setup/setup.go:297→315→349`). Tidy can change which version of a package is resolved — a user pinning `google.golang.org/grpc@v1.70.0` gets bumped to v1.82.0 because the grpc rules carry no `version:` range and the injected grpc hook module requires v1.82.0 — and the build then fails with `cannot replace ... during [compile ...]` because the recorded paths point at sources that no longer match what's fed to the compiler. The general mechanism is tracked upstream as [#494](https://github.com/open-telemetry/opentelemetry-go-compile-instrumentation/issues/494); full reproduction in [readiness finding 02](../readiness/evidence/findings/02-rule-matching-vs-tidy.md).

**Bundled rules mostly don't declare what versions they support.** A static audit of the bundled rule sets found only 4 of 22 carry any `version:` range (`docs/readiness/evidence/a2-static-version-audit.txt`), even though [ADR-0004](0004-instrumentation-ownership-and-compatibility.md) already commits the project to supporting "the last two major versions of each instrumented library." Nothing enforces that policy at the rule level today: an out-of-range library version is matched anyway, silently, and the resulting mismatch surfaces later as the same `cannot replace` failure or as a working-but-wrong-semantics binary rather than a clear "your version isn't supported" message.

**Every build injects the full bundled dependency set regardless of what the app uses.** The `init_sdk` file rule targets `main` unconditionally, pulling in `pkg/runtime` and its tree — grpc, prometheus client, x/net, and eight OTLP exporter modules — into every build, whether or not the app talks gRPC or scrapes Prometheus metrics. `go mod tidy` silently upgrades the user's own pinned versions of any of these to satisfy the injected requirements (`warnVersion`, `tool/internal/setup/sync.go:89-121`, prints a line to stdout and proceeds). Full reproduction in [readiness finding 03](../readiness/evidence/findings/03-silent-dependency-injection.md).

Whatever otelc does here by the time it tags v1 is the compatibility contract: loosening a silent behavior later is harmless, but tightening a silent behavior into an error, or narrowing what's always linked, is user-visible and breaking under the project's own v1 bar.

## Decision

Not yet decided. This ADR records four options raised during the readiness review for the SIG to evaluate; they address different parts of the problem above and are not mutually exclusive — more than one may be adopted.

### Option A — version-gate all bundled rules per the ADR-0004 support policy

Add `version:` ranges to every bundled rule, derived mechanically from the "last two major versions" policy ADR-0004 already commits to, and make an out-of-range match a reported skip (or a clear error) instead of a silent match that fails later, differently, and with a confusing message.

### Option B — re-match rules after dependency sync

Re-run dependency discovery and rule matching after `syncDeps`/`go mod tidy` complete, so recorded source paths always reflect the final resolved module graph. This is a mechanism correctness fix more than a policy choice — it closes finding 02's `cannot replace` failure by construction, independent of what version policy is chosen. [PR #655](https://github.com/open-telemetry/opentelemetry-go-compile-instrumentation/pull/655) implements this by moving dependency sync into the new `otelc pin` step and re-parsing compile commands afterward; **merged 2026-07-08** (`238f23d`), closing #494. Re-verified at `ad45522`: finding 02's repro now builds successfully with explicit "Bumped dependency"/"Bumped go version" notices — the bump is loud now, but still automatic, which is exactly the policy question Options A/C/D below decide.

### Option C — explicit failure instead of silent MVS bumps

When `go mod tidy` would raise the version of a library the user already directly depends on (as opposed to adding a new indirect dependency), fail the build with a specific message ("your grpc v1.70.0 is below the v1.82.0 required by otelc's grpc instrumentation; upgrade or disable the grpc integration") instead of printing a one-line warning and proceeding.

### Option D — split `pkg/runtime`'s exporter tree

Separate the always-injected SDK core from the grpc/prometheus/x/net-heavy exporter set, so an app that doesn't use OTLP-over-grpc or the Prometheus bridge doesn't get those modules injected (and doesn't pay their version-conflict risk, or their share of the ~1.5MB binary-size delta measured against orchestrion in `docs/readiness/benchmarks.md`). Exporter selection would gate which of the tree is linked, rather than `autoexport` linking all of it unconditionally.

## Consequences

**Option A.** Positive: directly implements a policy the project has already committed to (ADR-0004); turns a confusing build failure into a legible "unsupported version" diagnostic; the audit data needed to start (which rules currently lack ranges) already exists. Tradeoff: requires determining and maintaining accurate version ranges for each of the ~20 ungated rule sets, and deciding what happens to a build that has no supported version available (skip that instrumentation and continue, or fail the whole build).

**Option B.** Positive: fixes a real correctness bug with no user-visible downside — the build either works or fails for reasons unrelated to matching being stale; independent of any version-policy choice below. Tradeoff: a second dependency-resolution pass costs setup time (roughly another `go list`/dry-run pass per build); does not by itself add version gating (Option A) or change what's silently bumped (Option C).

**Option C.** Positive: replaces a silent, easy-to-miss stdout line with a decision point the user must act on before shipping a build that quietly carries different dependency versions than their own go.mod records; closes the SBOM/govulncheck blind spot where the shipped binary's dependencies diverge from what repo tooling audits. Tradeoff: a zero-config `otelc go build` can now fail where it previously succeeded, for a class of users who may not care about the version delta; needs a documented escape hatch (flag or env var) for users who want the old silent-upgrade behavior.

**Option D.** Positive: reduces binary size and dependency surface for the common case (an app that only needs, say, OTLP-over-http); removes a class of version conflicts entirely for apps that never wanted grpc or prometheus instrumentation in the first place. Tradeoff: the always-there SDK core needs a new, smaller default (likely OTLP-http-only) decided and documented; changes what "instrumented binary" means by default, which is itself a compat-sensitive choice best paired with the telemetry-defaults decision in [readiness finding 08](../readiness/evidence/findings/08-telemetry-defaults.md).

**Recommendation (non-binding).** Option B **landed upstream** (PR #655, merged `238f23d`), closing the mechanism half. Options A, C, and D are genuine policy choices with user-visible tradeoffs and are the ones that actually need SIG discussion before the v1 tag — #655 made the automatic version bump *loud* ("Bumped dependency …") but it remains automatic, so the policy question this ADR exists to settle is unchanged and now more urgent: whichever way it's decided becomes the compatibility contract at the tag.
