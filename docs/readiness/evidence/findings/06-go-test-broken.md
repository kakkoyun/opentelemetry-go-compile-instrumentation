# bug(tool): `otelc go test` fails for every package shape

Labels: `bug`
Suggested milestone: fix before v1, or remove `test` from the supported verbs until it works — shipping v1 with a broken advertised verb invites exactly the first-contact failures we want to avoid
Tested on: main @ 73f867f
Status: fix up for review as fork draft PR [kakkoyun#12](https://github.com/kakkoyun/opentelemetry-go-compile-instrumentation/pull/12) (see below) — not merged upstream

## What happens

`test` is an accepted subcommand (`tool/internal/setup/setup.go:541-546`), but no shape of `otelc go test` currently succeeds:

**Library package** — setup writes `otelc.runtime.go` with `package main` into the package directory:

```
found packages libpkg (lib.go) and main (otelc.runtime.go) in .../libpkg
FAIL    example.com/libpkg [setup failed]
```

**Package with tests in main** — two independent failures stack:

1. The generated `otelc.runtime.go` is not vet-clean, and `go test` runs vet by default:
   ```
   ./otelc.runtime.go:17:55: non-constant format string in call to log.Printf
   ```
2. With `-vet=off`, the synthetic test-main package that `go test` generates trips the `init_sdk` file rule's writer:
   ```
   failed to write to file $WORK/b001/otelc.init_otelsdk.go:
     format.Node internal error (6:1: expected 'IDENT', found 'import')
   ```
   (Same crash as `-cover` builds — see [finding 07](07-cover-broken-pgo-silent.md); likely one root cause in the file-rule AST writer when the target file already carries synthesized content.)

Repro is one command in any module: `otelc go test .`

Worth noting why CI never caught this: the integration suite builds app binaries with `otelc go build` and runs them; nothing in the repo exercises `otelc go test` itself.

## Suggested fix

- Don't write `otelc.runtime.go` into non-main packages; for test builds, the injection point should be the synthetic test-main (which is exactly what the reserved `test_main` target in the rule schema was going to address).
- Make generated code vet-clean (`log.Printf("%s", msg)` or `log.Print(msg)`).
- Fix the `format.Node` crash in the file-rule writer (shared with `-cover`).
- Add `otelc go test` (library + main package) to the integration matrix.

## Status update

A fix is up for review as fork draft PR [kakkoyun#12](https://github.com/kakkoyun/opentelemetry-go-compile-instrumentation/pull/12) (branch `v1-readiness/gotest-runtime-file`, rebased onto upstream `ad45522`): `otelc.runtime.go` is now generated with the target package's own name instead of a hardcoded `main`, the generated stack-print helper uses a vet-clean `Printf("%s", ...)` call, and the file-rule writer derives the package name from the compiled files themselves when setup-time resolution left it empty (synthetic test mains, cover-rewritten sources) instead of emitting an unparsable `package` clause. The `-cover` half of the shared crash is resolved by the companion stacked PR [kakkoyun#13](https://github.com/kakkoyun/opentelemetry-go-compile-instrumentation/pull/13) — see [finding 07](07-cover-broken-pgo-silent.md). Not merged upstream; treat the bug above as live until it lands.

Evidence: [b3-otelc-gotest-lib.log](../b3-otelc-gotest-lib.log), [b3-otelc-gotest-broken.txt](../b3-otelc-gotest-broken.txt), [b3-otelc-vet-bug.txt](../b3-otelc-vet-bug.txt).
