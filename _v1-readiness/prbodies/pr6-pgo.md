## Description

Removes the `-pgoprofile` skip from compile-command detection so PGO builds are instrumented like any other, and makes the nested import resolution profile-aware: `-pgo` is forwarded verbatim when explicit, and cmd/go's implicit `default.pgo` auto-detection is converted into an explicit `-pgo=<abspath>` for the nested `go list -export` (otherwise its archives are built without the profile and fail the link with a fingerprint mismatch).

## Motivation

With a `default.pgo` in the main package — increasingly standard practice — `otelc go build` exited 0 and produced a binary with zero instrumentation. No warning, no error; the user's telemetry just disappears. The skip dates to the original `isCompileCommand` and guarded a duplicate-import-path assertion that no longer exists; verified against the current dry-run plan: with a profile active, every package compiles exactly once with `-pgoprofile`, so there is nothing left to guard.

Known limitation, documented in code: multi-main builds with different per-main profiles can still mismatch on side-channel-resolved imports — the same restriction cmd/go documents for explicit `-pgo`, and it fails loudly, never silently.

## Verification

- Before: PGO app → exit 0, `Warning: no instrumentation will be applied`, 0 trampolines. After: 6 trampolines, spans emitted (client+server GET, shared trace ID). Without a profile: unchanged (6 trampolines). `-pgo=off` and `-pgo=custom.pgo` both work.
- Unit tests: PGO compile-arg detection, implicit-profile resolution (7 subtests), `-pgo` forwarding cases.
- `go test ./tool/...` green; gofmt clean; golangci-lint 0 issues.

---

## Checklist

- [x] PR title follows conventional commits format
- [x] Code formatted (gofmt)
- [x] Linters pass (golangci-lint run by author)
- [x] Tests pass: `go test ./tool/...`
- [x] Tests added for new functionality
- [x] Tests follow testing guidelines
- [ ] Documentation updated (n/a)
- [ ] OpenTelemetry Registry updated (n/a)
