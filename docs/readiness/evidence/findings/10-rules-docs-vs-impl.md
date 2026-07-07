# bug(docs/tool): documented `where` combinator syntax is rejected at rule load; schema must match implementation before the v1 freeze

Labels: `bug`, `scope:feat`
Suggested milestone: **v1 blocker in the compat sense** — the rule YAML schema is on the v1 freeze list (ADR-0003, [#261](https://github.com/open-telemetry/opentelemetry-go-compile-instrumentation/issues/261)); freezing it while docs and implementation disagree guarantees a breaking correction later
Tested on: main @ 73f867f

## What happens

docs/rules.md (line 111) states:

> Composition sub-groups `all-of`, `one-of`, `not` may appear at any position to compose nested selector groups.

A rule written exactly that way is rejected at load time:

```yaml
combinator_rule:
  target: net/http
  where:
    one-of:
      - func: Get
      - func: Post
  do:
    - inject_hooks: {before: BeforeRoundTrip, path: "..."}
```

```
Error: rule "combinator_rule" has no recognised selector
```

Two layers fail: rule-type inference can't classify the rule because it looks for known top-level fields rather than the `do:` modifier name ([#546](https://github.com/open-telemetry/opentelemetry-go-compile-instrumentation/issues/546)), and even past that, `where`-level combinators outside `where.file` return "not yet supported" (`tool/internal/setup/filter.go:185-199`). `has_directive` under `where.file` is likewise rejected (`filter.go:309`) though documented.

Also relevant for the GA decision: [#164](https://github.com/open-telemetry/opentelemetry-go-compile-instrumentation/issues/164) marks OP4.1–4.3 (intersection/union/negation combinators) as complete. They are complete for `where.file` only.

## Why this blocks v1 (per the compat bar)

The v1 freeze list locks the rule YAML schema. There are only two consistent options, and both are cheap now and expensive later:

- **Implement** general `where` combinators before the tag (they're already parsed and combinator machinery exists for `where.file`), or
- **De-document** them: state explicitly that combinators are `where.file`-only in v1, reserve the general syntax for a future minor, and make the load-time error say that ("combinators are not yet supported outside where.file") instead of "no recognised selector".

Shipping v1 with docs promising syntax the loader rejects means either breaking docs-compliant rule files later or breaking the schema.

While here: finishing [#546](https://github.com/open-telemetry/opentelemetry-go-compile-instrumentation/issues/546) (derive rule type from the `do:` modifier name) before the freeze would remove the "no recognised selector" failure class entirely, and it changes schema-validation behavior — another thing better done before v1 than after.

## Suggested fix

1. Decide implement-vs-de-document for general combinators; update docs/rules.md:111 and the parity checklist in [#164](https://github.com/open-telemetry/opentelemetry-go-compile-instrumentation/issues/164) to match reality either way.
2. Land [#546](https://github.com/open-telemetry/opentelemetry-go-compile-instrumentation/issues/546) before the freeze.
3. Improve the load-time errors to name the actual constraint.

## Status update

No fix is in progress as of this audit; both #546 and the `where`-level combinator implementation remain open. This finding is carried forward as one of the two things [ADR-0007 (draft)](../../../adr/0007-v1-compatibility-surface.md) flags as needing resolution before the rule-schema clause of the v1 freeze can be finalized.

Evidence: [a7-combinator.log](../a7-combinator.log), [a7-summary.txt](../a7-summary.txt).
