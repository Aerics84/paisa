package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func resetEpoch(t *testing.T) string {
	t.Helper()
	marker := filepath.Join(t.TempDir(), "paisa.db.synced")
	originalPath := epochPath
	epochPath = func() string { return marker }
	t.Cleanup(func() { epochPath = originalPath })

	epochMu.Lock()
	epochInit = false
	epochSeen = 0
	epochMu.Unlock()
	return marker
}

func seenEpoch() int64 {
	epochMu.Lock()
	defer epochMu.Unlock()
	return epochSeen
}

func TestClearIfStaleDetectsMarkerChange(t *testing.T) {
	marker := resetEpoch(t)

	// The first call only records the starting state.
	ClearIfStale()
	assert.Equal(t, int64(0), seenEpoch())

	// A marker that appears later is a change.
	MarkSynced()
	info, err := os.Stat(marker)
	assert.NoError(t, err)
	ClearIfStale()
	assert.Equal(t, info.ModTime().UnixNano(), seenEpoch())

	// No change, no new epoch.
	before := seenEpoch()
	ClearIfStale()
	assert.Equal(t, before, seenEpoch())

	// A second sync bumps the marker again.
	time.Sleep(20 * time.Millisecond)
	MarkSynced()
	ClearIfStale()
	assert.NotEqual(t, before, seenEpoch())
}
