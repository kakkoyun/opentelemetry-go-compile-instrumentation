// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.opentelemetry.io/otelc/tool/util"
)

func TestAcquireBuildLockExcludes(t *testing.T) {
	tmp := t.TempDir()
	t.Chdir(tmp)

	release1, err := AcquireBuildLock(t.Context())
	require.NoError(t, err)
	require.True(t, util.PathExists(util.GetBuildTemp(buildLockFileName)))

	// A second acquisition must not proceed while the first is held.
	var second atomic.Bool
	done := make(chan struct{})
	go func() {
		defer close(done)
		release2, err2 := AcquireBuildLock(context.Background())
		assert.NoError(t, err2)
		second.Store(true)
		release2()
	}()

	time.Sleep(3 * buildLockRetryInterval)
	assert.False(t, second.Load(), "second invocation must wait for the lock holder")

	release1()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("second invocation never acquired the lock after release")
	}
	assert.True(t, second.Load())
}

func TestAcquireBuildLockCancellable(t *testing.T) {
	tmp := t.TempDir()
	t.Chdir(tmp)

	release, err := AcquireBuildLock(t.Context())
	require.NoError(t, err)
	defer release()

	// A waiting acquisition must give up when its context is canceled —
	// this is what Ctrl-C during the wait resolves to.
	ctx, cancel := context.WithTimeout(context.Background(), 2*buildLockRetryInterval)
	defer cancel()
	_, err = AcquireBuildLock(ctx)
	require.Error(t, err)
}
