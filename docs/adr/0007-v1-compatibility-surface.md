# 7. v1 Compatibility Surface

Date: 2026-07-07

## Status

Draft / Proposed

## Context

Tagging v1 commits the project to SemVer: anything frozen at the tag can only change through a deprecation cycle afterward, while anything left explicitly internal can keep changing freely. The release roadmap ([#261](https://github.com/open-telemetry/opentelemetry-go-compile-instrumentation/issues/261)) already carries an informal "freeze checklist" — rule YAML schema, `HookContext`, CLI commands and flags, `OTELC_*` environment variables, and the `otel.instrumentation.go` config model — but it is a checklist inside a tracking issue, not a citable record of what's in scope and what's deliberately left out. This ADR proposes that record.

One item on the informal checklist is not ready to freeze as-is. `docs/rules.md` documents `where`-level composition (`all-of`/`one-of`/`not`) as available "at any position", but the loader rejects it outside `where.file` (`tool/internal/setup/filter.go:185-199`), and rule-type inference still falls back to field presence instead of the `do:` modifier name that ADR-0003 specifies ([#546](https://github.com/open-telemetry/opentelemetry-go-compile-instrumentation/issues/546)). The parity checklist in [#164](https://github.com/open-telemetry/opentelemetry-go-compile-instrumentation/issues/164) marks the combinators complete; that's true only for `where.file`. Full reproduction in [readiness finding 10](../readiness/evidence/findings/10-rules-docs-vs-impl.md). Freezing the rule schema while docs and implementation disagree guarantees breaking either docs-compliant rule files or the schema itself later — so the schema clause below is conditional on resolving that drift (implement general combinators, or de-document them) before the tag, not on this ADR's acceptance.

## Decision

Freeze the following at v1. Changing any of them afterward requires a deprecation cycle:

- **`HookContext`** ([ADR-0002](0002-api-design-and-project-structure.md)), defined at `pkg/hook/context.go:7-38` and copied into the toolexec-side template (`tool/internal/instrument/api.tmpl`) by `make build`/`build-all`/`install`, with drift caught by `make check-api-sync`. This is the parameter/return-value/data-passing contract every hook function is written against.
- **The rule YAML schema** — `target`/`version`/`where`/`do`/`imports`/`name` ([ADR-0003](0003-structured-rule-schema.md)) — conditional on resolving the combinator docs-vs-implementation drift described above first. Freezing before that resolution is not an option consistent with this ADR's own premise.
- **CLI verbs**: `otelc go build`, `otelc go install`, `otelc go test`, `otelc setup`, `otelc cleanup`, `otelc toolexec`, `otelc version` (`tool/cmd/otelc/cmd_*.go`), and — since [PR #655](https://github.com/open-telemetry/opentelemetry-go-compile-instrumentation/pull/655) merged (`238f23d`, 2026-07-08) — **`otelc pin`**, which therefore freezes as part of this list at the tag: its flags, its generated `otel.instrumentation.go` shape, and its go.mod-mutation semantics are now compat surface alongside the older verbs.
- **`OTELC_*` environment variables that a caller sets**: `OTELC_WORK_DIR` (backs `--work-dir`), `OTELC_RULES` (read directly in `tool/internal/setup/match.go:508`, backs `--rules`), `OTELC_STATS` and `OTELC_DEBUG` (back `--stats`/`--debug`) (`tool/util/shared.go:17-25`).
- **The `otel.instrumentation.go` convention** ([ADR-0005](0005-import-driven-instrumentation-selection.md)): the canonical filename and its `otelc.tool.go` alias (`tool/internal/setup/config.go:27-28`), the `//go:build tools` blank-import idiom, and the auto-pin fallback when no tool file is present.

Explicitly internal — implementation detail, free to change without notice:

- **`matched.json`**, the serialized matched rule set passed from setup to the instrument phase (`tool/util/shared.go:33-34`, written at `tool/internal/setup/setup.go:355` and `tool/internal/setup/store.go:89`).
- **`state.json`**, the crash-recovery manifest (`tool/internal/setup/state.go:22`).
- **The `.otelc-build/` layout** (`tool/util/shared.go:26`), including the persistent `gocache/` subdirectory and the extracted module directories used for `replace` targets.
- **The embedded bundle format**, currently a `tgz` (`tool/data/export.go:10`, `//go:embed otelc-bundle.tgz`) containing the `pkg/` and `instrumentation/` trees.
- **`added_imports.*.json`** per-process import-tracking files (`tool/util/shared.go:38-48`).
- **`OTELC_BUILD_FLAGS`**, which despite the `OTELC_*` prefix is not a setting a caller is expected to set: `GoBuild` writes it (`tool/internal/setup/setup.go:515`) to hand the JSON-encoded build flags to a nested process, and `GetBuildFlags` (`tool/util/shared.go:74`) reads it back. It's IPC between otelc's own processes, not configuration.

## Consequences

- Contributors get a single citable answer to "can I change this without a deprecation cycle" instead of having to infer it from the `#261` checklist or from ADR-0002/0003/0005 individually.
- The internal list is a deliberate invitation to keep improving the implementation freely: the crash-safety and cache-identity fixes referenced in `docs/readiness/README.md` ([findings 01](../readiness/evidence/findings/01-build-cache-staleness.md) and [04](../readiness/evidence/findings/04-interrupted-build-bricks-project.md)) both change `state.json`'s and the toolexec cache stamp's internal shape, and neither would be a compatibility question under this ADR.
- The conditional rule-schema clause means this ADR cannot be treated as fully "Accepted" until the combinator drift (finding 10) is resolved one way or the other; accepting this ADR before then would freeze a schema the project's own docs and loader still disagree about.
- `otelc pin` is named here as a candidate frozen verb specifically so its interface (flags, file format it writes) gets the same v1 scrutiny as the rest of the CLI if it lands before the tag, rather than slipping in unreviewed because it arrived late.
