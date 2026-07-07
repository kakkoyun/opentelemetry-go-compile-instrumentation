// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"context"
	"os"
	"time"

	"github.com/gofrs/flock"

	"go.opentelemetry.io/otelc/tool/ex"
	"go.opentelemetry.io/otelc/tool/util"
)

const (
	buildLockFileName = "build.lock"
	// buildLockRetryInterval is how often a waiting invocation re-attempts the
	// lock. The wait itself is unbounded: it ends when the holder finishes or
	// the caller cancels (Ctrl-C), and a log line makes the waiting visible.
	buildLockRetryInterval = 200 * time.Millisecond
)

// AcquireBuildLock serializes otelc invocations that mutate the module —
// setup, build, and cleanup all rewrite go.mod/go.sum and share
// .otelc-build/. Without the lock, a second concurrent build snapshots the
// first build's already-mutated go.mod as its "original" and the restore
// step then bakes the replace directives in permanently.
//
// The lock is an OS advisory file lock, so it is released automatically if
// the process dies, however it dies.
//
// The returned release function must be called (deferred) by the caller.
func AcquireBuildLock(ctx context.Context) (func(), error) {
	if err := os.MkdirAll(util.GetBuildTempDir(), 0o755); err != nil {
		return nil, ex.Wrapf(err, "creating build temp dir for lock")
	}

	lock := flock.New(util.GetBuildTemp(buildLockFileName))

	acquired, err := lock.TryLock()
	if err != nil {
		return nil, ex.Wrapf(err, "acquiring build lock %s", lock.Path())
	}
	if !acquired {
		logger := util.LoggerFromContext(ctx)
		logger.InfoContext(ctx, "another otelc invocation holds the build lock; waiting",
			"path", lock.Path())
		if _, err = lock.TryLockContext(ctx, buildLockRetryInterval); err != nil {
			return nil, ex.Wrapf(err, "waiting for build lock %s", lock.Path())
		}
	}

	return func() {
		_ = lock.Unlock()
	}, nil
}
