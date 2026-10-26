# Benchmarks vs orchestrion

Comparison of otelc against DataDog's orchestrion, run as part of the v1 readiness review in `docs/readiness/README.md`. Goal of the comparison: check whether otelc's build-time and runtime cost are in the range users of a comparable compile-time instrumentation tool already accept.

## Setup

- Target app: a small `net/http` client+server app, one round trip per invocation.
- Environment: single 4-core Linux container, go1.25.0. Both tools built at the versions in use during this audit: otelc @73f867f, orchestrion v1.11.0 with dd-trace-go v2.9.1's `net/http` contrib instrumentation enabled.
- Both sides run with noop export pipelines, but the two tools reach "noop" by different means:
  - orchestrion/dd-trace-go: `DD_TRACE_ENABLED=false`, an explicit, documented kill switch.
  - otelc: no `OTEL_EXPORTER_OTLP_ENDPOINT` (or `..._TRACES_ENDPOINT`) is set, which leaves traces off as a side effect of the default-off behavior described in [finding 08](evidence/findings/08-telemetry-defaults.md), not an explicit opt-out. See "Caveats" below.
- Build timings: two cold-cache runs and five warm-cache runs (`touch main.go` between each) per tool; cold is reported as the average of the two runs, warm as the median of the five.
- Runtime timings (goroutine spawn, HTTP round trip): three repeated rounds per tool; the minimum observed value per metric per tool is reported, which is standard practice for reducing scheduler/OS noise in microbenchmarks.

## Build time and binary size

| Metric | plain | otelc | orchestrion |
|---|---|---|---|
| Cold build | 10.7s | 26.1s (2.4x) | 23.4s (2.2x) |
| Warm rebuild (touch main.go) | 0.09s | 2.3s (1.3-2.6s across runs) | 2.8s |
| Binary size | 8.9MB | 25.2MB (2.8x) | 23.7MB (2.7x) |

Raw data: [b1-plain.txt](evidence/b1-plain.txt), [b1-otelc.txt](evidence/b1-otelc.txt), [b1-orch.txt](evidence/b1-orch.txt).

otelc's warm-rebuild cost is already slightly below orchestrion's — both pay a fixed 2.3-2.8s per rebuild regardless of how small the change is. For otelc that fixed cost is its per-build setup phase (`go build -a -x -n` dry run, bundle extraction, `go mod tidy`); it is the clearest target if the project wants to be not just at parity but faster than orchestrion on the common inner-loop case. Cold build is about 12% slower than orchestrion; binary size carries roughly 1.5MB over orchestrion, attributable to otelc always linking every OTLP/prometheus/stdout exporter via autoexport rather than only the one in use (see [finding 03](evidence/findings/03-silent-dependency-injection.md)).

## Runtime overhead

Noop pipelines (see setup above); minimum of 3 rounds per metric.

| Metric | plain | otelc | orchestrion |
|---|---|---|---|
| Goroutine spawn | 469 ns/op | 468 ns/op | 461 ns/op |
| HTTP round trip | 81.5 us/op | 95.7 us/op (+17%) | 94.6 us/op (+16%) |

Raw data: [b3-runtime.txt](evidence/b3-runtime.txt), [b3-plain.txt](evidence/b3-plain.txt).

Goroutine spawn cost is indistinguishable across all three (otelc's goroutine-local-storage hook on `runtime.newproc1` adds no measurable overhead). HTTP round-trip overhead is at parity with orchestrion, both roughly +16-17% over plain in this noop configuration.

## Caveats

- **Single shared container, single small app.** No isolation from noisy neighbors, and the app exercises exactly one code path (`net/http` client + server). The numbers are directional, not a general overhead SLO; that kind of SLO and its CI enforcement are tracked separately in [issue #570](https://github.com/open-telemetry/opentelemetry-go-compile-instrumentation/issues/570), and this write-up is meant to give that effort a starting methodology.
- **otelc's "noop" is not an explicit setting.** As noted above, otelc reaches zero trace export by relying on the endpoint-absent default being off — one of the [telemetry-defaults finding](evidence/findings/08-telemetry-defaults.md)'s open decisions. If that default changes (for example, to a spec-compliant default endpoint), this comparison's otelc baseline would need an explicit disable flag to stay a true noop measurement. orchestrion's `DD_TRACE_ENABLED=false` has no such dependency.
- **Different SDK semantics in noop mode.** otelc still initializes the full OpenTelemetry SDK object graph (providers, resource detection) even with no exporter reachable; orchestrion/dd-trace-go's disabled mode may skip more of its own setup. The comparison measures each tool's own idea of "off", not an identical code path.
- **Cold-build asymmetry suggests a caching difference, not just tool overhead.** orchestrion's two cold runs were 56.1s and 23.4s — the first run visibly paid a one-time download cost that the second did not. otelc's two cold runs were 25.9s and 26.2s, nearly identical. The reported orchestrion cold-build figure (23.4s) excludes its download-tainted first run to make the comparison fairer, but the fact that otelc shows no equivalent first-run tax suggests its module cache may already have been warm from earlier phases of this same audit session. The comparison does not independently control for this, so the cold-build multiplier (2.2x-2.4x) should be read as approximate.
