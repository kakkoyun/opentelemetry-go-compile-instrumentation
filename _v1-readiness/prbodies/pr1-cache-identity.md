## Description

Answers the `compile -V=full` / `link -V=full` probes that cmd/go issues through the toolexec wrapper with the underlying tool's version string extended by `otelc@<version>;rules=<digest>`, where the digest covers the stored matched rule set. Sorts matched rulesets and per-file rule parsing so `matched.json` is byte-identical for identical input. Runs the nested `go list -export` (used to resolve instrumentation-added imports) through the same toolexec so its archives share the stamped action IDs, with an env guard that stops that nested resolve from recursing.

## Motivation

Nothing about otelc was part of the build-cache key. cmd/go computes action IDs from the original sources and tool IDs before toolexec runs, so with a warm cache:

- adding rules silently reused uninstrumented archives — the binary built, ran, and emitted nothing (worst case, reproduced);
- removing rules failed at link with `relocation target ...BeforeRoundTrip not defined` against stale instrumented archives (reproduced);
- a user-set GOCACHE shared with plain builds poisoned `go build`: after one otelc build, a plain build in the same module failed at link (reproduced — this is the common CI shape).

Orchestrion solves the same problem the same way (appending `orchestrion@version;aspects=sha512:...` to `-V=full` output).

Repro before the fix, silent-loss direction:

```sh
touch empty-rules.yaml
OTELC_RULES=$PWD/empty-rules.yaml otelc go build -o app-v1 .   # cold cache, no rules
otelc go build -o app-v2 .                                     # default rules, warm cache
go tool nm app-v2 | grep -c OtelBeforeTrampoline               # 0, should be >0
```

## Verification

- New unit tests: `TestIsToolIDProbe`, `TestMatchedRulesDigest`, `TestStampToolID` (runs the real compile tool, asserts single-line output, digest sensitivity).
- `go test ./tool/...` green; gofmt clean.
- End-to-end: all three repro directions flipped — rules-added rebuild now instruments (6 trampolines + init logs), rules-removed rebuild produces a clean binary instead of a link failure, and a plain `go build` sharing the user GOCACHE succeeds with 0 trampolines while the otelc binary has 6.
- Warm same-rules rebuild stays incremental (~2.9s, versus ~26s cold), so cache hits are preserved.

---

## Checklist

- [x] PR title follows conventional commits format
- [x] Code formatted (gofmt)
- [ ] Linters pass: `make lint` (please run in CI)
- [x] Tests pass: `go test ./tool/...`
- [x] Tests added for new functionality
- [x] Tests follow testing guidelines
- [ ] Documentation updated (n/a)
- [ ] OpenTelemetry Registry updated (n/a)
