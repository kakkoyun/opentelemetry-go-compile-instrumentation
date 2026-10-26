# bug(tool): a killed build leaves go.mod rewritten and the project unbuildable; `otelc cleanup` cannot recover (state is never committed)

Labels: `bug`
Suggested milestone: fix before v1 — CI job cancellations and OOM kills are routine, and the failure mode is a broken working tree
Tested on: main @ 73f867f
Status: fix up for review as fork draft PR [kakkoyun#10](https://github.com/kakkoyun/opentelemetry-go-compile-instrumentation/pull/10) (see below) — not merged upstream

## What happens

`StateManager` snapshots go.mod/go.sum before mutation, but in the `otelc go build` path `Commit()` is never called — `GoBuild` places the manager in the context (`tool/internal/setup/setup.go:530`) and `Setup` skips the commit branch when it finds one there (`setup.go:321-332`). The only `Commit()` call site is the standalone-`otelc setup` path. So `state.json` is never written and `.otelc-build/state/` stays empty during normal builds.

After `kill -9` mid-build (equivalently: OOM kill, CI cancellation, power loss):

1. go.mod keeps 8 `replace` directives pointing into `.otelc-build/`, go.sum and `otelc.runtime.go` are orphaned in the tree.
2. `otelc cleanup` loads nil state (`LoadStateManager` returns `(nil, nil)` when `state.json` is absent, `state.go:48-52`), restores nothing — and deletes the extracted module directories, so the dangling replaces now point at nothing. It also errors: `failed to remove build temp dir ... directory not empty`.
3. Plain `go build`: `replacement directory .../.otelc-build/instrumentation/... does not exist`.
4. Retry `otelc go build`: fails on `missing go.sum entry` for the injected modules.

The only way out is manual: `git checkout go.mod && rm otelc.runtime.go go.sum`. A user without the files under version control loses their original go.mod contents' integrity guarantees entirely.

## Repro

```sh
# any app; start a build and kill the process group once go.mod has been rewritten
setsid otelc go build -o app . & BPID=$!
until grep -q "^replace" go.mod; do sleep 0.1; done; sleep 2
kill -9 -$BPID

grep -c "^replace" go.mod        # 8
otelc cleanup                    # restores nothing, deletes replace targets
go build -o app .               # replacement directory ... does not exist
otelc go build -o app .         # missing go.sum entry for module ...
```

## Suggested fix

- Call `Commit()` immediately after `TrackAll` — before the first mutation, not after all of them — in every path that mutates the tree, including `GoBuild`. The current ordering leaves a window even in the path that does commit.
- Make `otelc cleanup` the guaranteed recovery story: with a manifest on disk it can restore go.mod/go.sum and remove `otelc.runtime.go` no matter how the previous build died. It should also refuse to delete `.otelc-build` contents that a dirty go.mod still references, or fix go.mod first.
- Add a crash-recovery test: kill -9 between `syncDeps` and the toolexec build, then assert `otelc cleanup` (or the next build) restores a working tree.

Related: the same missing-manifest problem makes concurrent builds destructive ([finding 05](05-concurrent-builds-corrupt-gomod.md)).

## Status update

A fix along these lines is up for review as fork draft PR [kakkoyun#10](https://github.com/kakkoyun/opentelemetry-go-compile-instrumentation/pull/10) (branch `v1-readiness/state-commit`, rebased onto upstream `ad45522`): `Track` now writes the manifest atomically (temp file + rename, sorted entries) before returning in both setup paths, and `cleanup` restores from that manifest in a fresh process, discarding consumed state after a successful revert and refusing to delete `.otelc-build` while a failed revert still needs the snapshots inside it. Not merged upstream; treat the bug above as live until it lands. Upstream context: the reviewer discussion on [#659](https://github.com/open-telemetry/opentelemetry-go-compile-instrumentation/pull/659) independently confirms the persistence gap this fixes.

Evidence: [a3-summary.txt](../a3-summary.txt), [a3-rebuild-after-kill-fails.log](../a3-rebuild-after-kill-fails.log), [a2-gomod-during-build.txt](../a2-gomod-during-build.txt).
