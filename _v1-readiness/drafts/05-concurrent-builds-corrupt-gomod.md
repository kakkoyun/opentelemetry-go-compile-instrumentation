# bug(tool): two concurrent `otelc go build` runs in one module permanently bake replace directives into go.mod

Labels: `bug`
Suggested milestone: fix before v1 (data-loss class; IDE save-hooks and parallel CI matrix jobs will trigger it)
Tested on: main @ 73f867f

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
- The commit-before-mutate fix from the interrupted-build issue also stops the mutated-snapshot problem: with a manifest on disk, the second build can detect that go.mod is already in a mutated state and refuse (or restore first).

Evidence: `a4-concurrent-builds.txt`.
