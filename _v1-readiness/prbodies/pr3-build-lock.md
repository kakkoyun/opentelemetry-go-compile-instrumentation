## Description

Takes an OS advisory file lock (`gofrs/flock` on `.otelc-build/build.lock`) around every otelc invocation that mutates the module: `go build/install/test`, `setup`, `cleanup`. A second invocation logs that it is waiting and blocks until the holder finishes or its context is canceled. Advisory locks die with the process, so a killed holder cannot wedge the module.

## Motivation

Nothing serialized concurrent otelc runs in one module. A second build starting mid-setup snapshotted the first build's already-mutated go.mod as its "original"; whichever build finished last restored that mutated version, baking the replace directives in permanently. The runs also raced on `.otelc-build` (import-tracking files deleted across builds, `matched.json` clobbered). Reproduced: two parallel `otelc go build` runs → build A fails, build B "succeeds", go.mod left with all 8 replace directives. IDE save-hooks and parallel CI jobs hit this shape naturally.

New dependency: `github.com/gofrs/flock` (BSD-3, no transitive deps). A hand-rolled `O_EXCL` PID file was the alternative; flock was chosen because stale-lock handling on all three supported OSes is exactly the fiddly part.

## Verification

- New tests: lock exclusion between concurrent acquisitions, cancellation while waiting.
- `go test ./tool/...` green.
- End-to-end: the concurrent-build repro flipped — both builds exit 0 and go.mod is pristine afterward.

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
