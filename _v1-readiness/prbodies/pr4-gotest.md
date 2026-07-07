## Description

Makes `otelc go test` work. Three fixes: generate `otelc.runtime.go` with the target package's own name instead of a hardcoded `package main`; emit vet-clean generated code (`log.Printf("%s", ...)`); derive the package name for file rules from the files actually being compiled when setup-time resolution left it empty (synthetic test mains, cover-rewritten sources), instead of writing an unparsable `package ` clause.

## Motivation

`test` is an accepted verb, but no shape of `otelc go test` succeeded:

- Library package: `found packages libpkg (lib.go) and main (otelc.runtime.go)` → setup failed.
- Main package: `./otelc.runtime.go:17:55: non-constant format string in call to log.Printf` (go test runs vet by default); with `-vet=off`, the synthetic test-main crashed with `format.Node internal error (6:1: expected 'IDENT', found 'import')` in the init_sdk file rule.

Repo CI never calls `otelc go test` (integration tests build binaries), which is how all three survived. The same empty-package-name crash affects `-cover` builds; that path additionally needs the cache-identity fix (companion PR) and is verified there.

## Verification

- Updated golden files for the generated runtime file (only the Printf form changed).
- `go test ./tool/...` green; gofmt clean.
- End-to-end: `otelc go test .` passes on a library package (was: setup failed) and on a main package with tests (was: vet failure, then format.Node crash).

Follow-up worth doing: add `otelc go test` (library + main) to the integration matrix.

---

## Checklist

- [x] PR title follows conventional commits format
- [x] Code formatted (gofmt)
- [ ] Linters pass: `make lint` (please run in CI)
- [x] Tests pass: `go test ./tool/...`
- [x] Tests added for new functionality (golden + crash tests)
- [x] Tests follow testing guidelines
- [ ] Documentation updated (n/a)
- [ ] OpenTelemetry Registry updated (n/a)
