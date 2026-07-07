## Description

Makes `StateManager.Track` persist the state manifest atomically (temp file + rename, sorted entries) before returning, in every path. Adds `Discard` for consumed state. `Cleanup` now restores from the on-disk manifest in a fresh process, discards state after a successful revert, and refuses to delete `.otelc-build` while a failed revert still needs the snapshots inside it.

## Motivation

The manifest was only written by standalone `otelc setup`, after all mutations; the `otelc go build` path never called `Commit` at all. Killing a build after go.mod was rewritten (SIGKILL, OOM kill, CI cancellation) left the module bricked, and recovery tooling made it worse — reproduced end to end:

```
kill -9 <build pgroup mid-build>
grep -c "^replace" go.mod            # 8 — points into .otelc-build/
otelc cleanup                        # restores nothing (nil state), deletes replace targets
go build .                          # replacement directory ... does not exist
otelc go build .                    # missing go.sum entry for module ...
```

Only manual `git checkout go.mod && rm otelc.runtime.go go.sum` recovered the tree.

Note: #538 (per-module backups) touches the same files; happy to rebase whichever lands second.

## Verification

- New tests in `state_crash_test.go`: fresh-process recovery from the manifest alone (no explicit Commit), cleanup discarding consumed state, cleanup preserving snapshots when revert fails, atomic + deterministic manifest writes.
- `go test ./tool/...` green; gofmt clean.
- End-to-end: same kill -9 repro now recovers — `state.json` exists at kill time, `otelc cleanup` restores go.mod (0 replaces), removes `otelc.runtime.go`, and plain `go build` works again.

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
