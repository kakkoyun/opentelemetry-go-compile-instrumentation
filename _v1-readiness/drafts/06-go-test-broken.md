# bug(tool): `otelc go test` fails for every package shape

Labels: `bug`
Suggested milestone: fix before v1, or remove `test` from the supported verbs until it works — shipping v1 with a broken advertised verb invites exactly the first-contact failures we want to avoid
Tested on: main @ 73f867f

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
   (Same crash as `-cover` builds — see companion issue; likely one root cause in the file-rule AST writer when the target file already carries synthesized content.)

Repro is one command in any module: `otelc go test .`

Worth noting why CI never caught this: the integration suite builds app binaries with `otelc go build` and runs them; nothing in the repo exercises `otelc go test` itself.

## Suggested fix

- Don't write `otelc.runtime.go` into non-main packages; for test builds, the injection point should be the synthetic test-main (which is exactly what the reserved `test_main` target in the rule schema was going to address).
- Make generated code vet-clean (`log.Printf("%s", msg)` or `log.Print(msg)`).
- Fix the `format.Node` crash in the file-rule writer (shared with `-cover`).
- Add `otelc go test` (library + main package) to the integration matrix.

Evidence: `b3-otelc-gotest-lib.log`, `b3-otelc-gotest-broken.txt`.
