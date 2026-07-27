# bug(tool): two concurrent `otelc go build` runs in one module permanently bake replace directives into go.mod

Labels: `bug`
Suggested milestone: fix before v1 (data-loss class; IDE save-hooks and parallel CI matrix jobs will trigger it)
Tested on: main @ 73f867f, go1.25.0, linux/amd64
Status: fix **submitted upstream as [open-telemetry#672](https://github.com/open-telemetry/opentelemetry-go-compile-instrumentation/pull/672)** (from fork PR [kakkoyun#11](https://github.com/kakkoyun/opentelemetry-go-compile-instrumentation/pull/11); see below) — awaiting review

## What happens

There is no locking around `.otelc-build` or the go.mod mutation window. When a second build starts while the first is in its setup phase, the second `Setup` snapshots the **already-mutated** go.mod as its "original". Whichever build finishes last "restores" that mutated version, and the replace directives + injected requires become permanent. The first build usually also fails mid-flight because the two runs fight over `.otelc-build` (extracted modules, `matched.json`, import-tracking files — `CleanupImportTrackingFiles` in `tool/internal/instrument/toolexec.go:224-234` deletes the other build's files).

Observed: build A fails with `could not import go.opentelemetry.io/otelc/instrumentation/log/slog (open : no such file or directory)`; build B exits 0; afterwards go.mod retains all 8 replace directives and the orphaned go.sum, in a "successful" state that no longer builds without `.otelc-build` present.

## Repro

```sh
otelc go build -o a . & sleep 1.5
otelc go build -o b . &
wait
grep -c "^replace" go.mod   # 8 — should be 0 after both builds complete
```

## Suggested fix

- Take an exclusive lock (flock on a file under `.otelc-build/`) around the whole setup→build→restore span; make the second invocation wait or fail fast with a clear message.
- Namespace per-build artifacts (`matched.json`, import-tracking files) by build ID rather than per-directory, so a stray concurrent invocation can't delete another run's state.
- The commit-before-mutate fix from [finding 04](04-interrupted-build-bricks-project.md) also stops the mutated-snapshot problem: with a manifest on disk, the second build can detect that go.mod is already in a mutated state and refuse (or restore first).

## Status update

A fix is up for review as fork draft PR [kakkoyun#11](https://github.com/kakkoyun/opentelemetry-go-compile-instrumentation/pull/11) (branch `v1-readiness/build-lock`, rebased onto upstream `ad45522`): an OS advisory file lock (`gofrs/flock`) now wraps every invocation that mutates the module — `go build`/`install`/`test`, `setup`, and `cleanup`. A second invocation logs that it is waiting and blocks until the holder finishes or its own context is canceled; advisory locks die with the process, so a killed holder cannot wedge the module. The before/after repro matches the one above (8 replace directives after two parallel builds, before; both succeed and go.mod is restored, after). Fork CI subsequently caught and the PR fixed two portability details worth reviewing: the lock file lives *next to* `.otelc-build`, not inside it (Windows cannot delete a directory containing the process's own open lock file), and the sibling `test/` module needed re-tidying for flock's `golang.org/x/sys` bump. Not merged upstream; treat the bug above as live until it lands.

Evidence: [a4-concurrent-builds.txt](../a4-concurrent-builds.txt).
