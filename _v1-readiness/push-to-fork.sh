#!/usr/bin/env bash
# Push the v1-readiness branches to the fork and open draft PRs.
# Usage: ./push-to-fork.sh /path/to/v1-readiness.bundle
set -euo pipefail
BUNDLE=${1:?usage: push-to-fork.sh <bundle>}
FORK=git@github.com:kakkoyun/opentelemetry-go-compile-instrumentation.git

TMP=$(mktemp -d)
git clone "$FORK" "$TMP/fork"
cd "$TMP/fork"
git fetch upstream 2>/dev/null || git remote add upstream https://github.com/open-telemetry/opentelemetry-go-compile-instrumentation.git
git fetch "$OLDPWD/$BUNDLE" \
  'refs/heads/v1-readiness/*:refs/heads/v1-readiness/*' \
  refs/heads/claude/otelc-v1-readiness-ka0zpn:refs/heads/claude/otelc-v1-readiness-ka0zpn

for b in v1-readiness/cache-identity v1-readiness/state-commit v1-readiness/build-lock v1-readiness/gotest-runtime-file v1-readiness/cover-fingerprint v1-readiness/pgo-not-silent v1-readiness/sdk-disabled v1-readiness/readiness-report; do
  git push -u origin "$b"
done
git push -u origin claude/otelc-v1-readiness-ka0zpn   # session snapshot incl. report/evidence

# Draft PRs on the fork (base = fork's main). PR bodies live next to this script.
gh pr create --repo kakkoyun/opentelemetry-go-compile-instrumentation --draft --base main \
  --head v1-readiness/cache-identity \
  --title "fix(tool): stamp compile/link tool IDs with otelc version and rules digest" \
  --body-file "$OLDPWD/pr1-cache-identity.md"
gh pr create --repo kakkoyun/opentelemetry-go-compile-instrumentation --draft --base main \
  --head v1-readiness/state-commit \
  --title "fix(setup): persist state before mutating the tree; make cleanup crash-safe" \
  --body-file "$OLDPWD/pr2-state-commit.md"
gh pr create --repo kakkoyun/opentelemetry-go-compile-instrumentation --draft --base main \
  --head v1-readiness/build-lock \
  --title "fix(setup): serialize concurrent otelc invocations with a build lock" \
  --body-file "$OLDPWD/pr3-build-lock.md"
gh pr create --repo kakkoyun/opentelemetry-go-compile-instrumentation --draft --base main \
  --head v1-readiness/gotest-runtime-file \
  --title "fix(tool): make otelc go test work for library and main packages" \
  --body-file "$OLDPWD/pr4-gotest.md"
gh pr create --repo kakkoyun/opentelemetry-go-compile-instrumentation --draft --base main \
  --head v1-readiness/cover-fingerprint \
  --title "fix(tool): stop forwarding -cover to nested go list -export resolves" \
  --body-file "$OLDPWD/pr5-cover-fingerprint.md"
gh pr create --repo kakkoyun/opentelemetry-go-compile-instrumentation --draft --base main \
  --head v1-readiness/pgo-not-silent \
  --title "fix(tool): instrument PGO builds instead of silently skipping them" \
  --body-file "$OLDPWD/pr6-pgo.md"
gh pr create --repo kakkoyun/opentelemetry-go-compile-instrumentation --draft --base main \
  --head v1-readiness/sdk-disabled \
  --title "fix(runtime): honor OTEL_SDK_DISABLED and export traces/logs like metrics" \
  --body-file "$OLDPWD/pr7-sdk-disabled.md"
gh pr create --repo kakkoyun/opentelemetry-go-compile-instrumentation --draft --base main \
  --head v1-readiness/readiness-report \
  --title "docs: add v1 readiness report, evidence, and two draft ADRs" \
  --body "Data-backed v1 readiness review: report, reproduction evidence for every finding, orchestrion benchmark comparison, and draft ADRs 0006 (dependency version policy) and 0007 (v1 compatibility surface). Produced alongside the companion fix PRs; every claim links to an evidence file. Intended for SIG review before the v1 tag."
