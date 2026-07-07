// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.opentelemetry.io/otelc/tool/util"
)

// These tests simulate a build process dying (SIGKILL, OOM, CI cancellation)
// between mutating the user's tree and reverting it. Recovery must work from
// the on-disk manifest alone, in a fresh process, with no in-memory state.

func TestTrackPersistsBeforeMutation(t *testing.T) {
	tmp := t.TempDir()
	t.Chdir(tmp)

	goMod := filepath.Join(tmp, "go.mod")
	mustWriteFile(t, goMod, "module example.com/app\n\ngo 1.25.0\n")
	runtimeFile := filepath.Join(tmp, OtelcRuntimeFile)

	// The "build" process tracks, mutates, and dies without any explicit
	// Commit — exactly the `otelc go build` path.
	dying := NewStateManager()
	require.NoError(t, dying.Track(goMod))
	require.NoError(t, dying.Track(runtimeFile))
	mustWriteFile(t, goMod, "module example.com/app\n\ngo 1.25.0\n\nreplace x => ./.otelc-build/x\n")
	mustWriteFile(t, runtimeFile, "package main\n")
	// (process dies here; `dying` is gone)

	// A fresh process recovers from the manifest alone.
	recovered, err := LoadStateManager()
	require.NoError(t, err)
	require.NotNil(t, recovered, "manifest must exist without an explicit Commit")
	require.NoError(t, recovered.Revert())

	content, err := os.ReadFile(goMod)
	require.NoError(t, err)
	assert.Equal(t, "module example.com/app\n\ngo 1.25.0\n", string(content), "go.mod must be restored")
	assert.False(t, util.PathExists(runtimeFile), "generated runtime file must be removed")
}

func TestCleanupRecoversAfterCrash(t *testing.T) {
	tmp := t.TempDir()
	t.Chdir(tmp)

	goMod := filepath.Join(tmp, "go.mod")
	mustWriteFile(t, goMod, "module example.com/app\n")

	dying := NewStateManager()
	require.NoError(t, dying.Track(goMod))
	mustWriteFile(t, goMod, "module example.com/app\n\nreplace x => ./.otelc-build/x\n")

	// Fresh process, no state manager in the context: Cleanup must load the
	// manifest from disk, restore the tree, and discard the consumed state.
	require.NoError(t, Cleanup(t.Context(), false))

	content, err := os.ReadFile(goMod)
	require.NoError(t, err)
	assert.Equal(t, "module example.com/app\n", string(content))
	assert.False(t, util.PathExists(util.GetBuildTemp(stateFileName)), "consumed manifest must be discarded")
	assert.False(t, util.PathExists(util.GetBuildTemp(stateDir)), "consumed snapshots must be discarded")
}

func TestCleanupKeepsSnapshotsWhenRevertFails(t *testing.T) {
	tmp := t.TempDir()
	t.Chdir(tmp)

	goMod := filepath.Join(tmp, "go.mod")
	otherFile := filepath.Join(tmp, "other.txt")
	mustWriteFile(t, goMod, "module example.com/app\n")
	mustWriteFile(t, otherFile, "keep me\n")

	dying := NewStateManager()
	require.NoError(t, dying.Track(goMod))
	require.NoError(t, dying.Track(otherFile))

	// Sabotage one snapshot so Revert fails partially.
	require.NoError(t, os.Remove(filepath.Join(util.GetBuildTemp(stateDir), stateSnapshotPath(otherFile))))

	require.NoError(t, Cleanup(t.Context(), true))

	// Even with cleanAll, a failed revert must not delete the remaining
	// snapshots — they are the only path left to recovery.
	assert.True(t, util.PathExists(util.GetBuildTemp(stateFileName)),
		"manifest must survive a failed revert even with cleanAll")
	assert.True(t, util.PathExists(filepath.Join(util.GetBuildTemp(stateDir), stateSnapshotPath(goMod))),
		"snapshots must survive a failed revert even with cleanAll")
}

func TestCommitIsAtomicAndSorted(t *testing.T) {
	tmp := t.TempDir()
	t.Chdir(tmp)

	b := filepath.Join(tmp, "b.txt")
	a := filepath.Join(tmp, "a.txt")
	mustWriteFile(t, a, "a")

	s := NewStateManager()
	require.NoError(t, s.Track(b)) // missing file, tracked first
	require.NoError(t, s.Track(a))

	first, err := os.ReadFile(util.GetBuildTemp(stateFileName))
	require.NoError(t, err)
	require.NoError(t, s.Commit())
	second, err := os.ReadFile(util.GetBuildTemp(stateFileName))
	require.NoError(t, err)
	assert.Equal(t, string(first), string(second), "manifest serialization must be deterministic")
	assert.False(t, util.PathExists(util.GetBuildTemp(stateFileName)+".tmp"), "no temp file left behind")
}
