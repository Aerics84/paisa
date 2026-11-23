package pricesync

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileStateStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "paisa.db.price-update.json")
	store := NewFileStateStore(path)
	want := State{
		SchemaVersion:        1,
		LastAttemptAt:        time.Date(2026, 11, 20, 22, 30, 0, 0, time.UTC),
		LastSuccessAt:        time.Date(2026, 11, 20, 22, 30, 31, 0, time.UTC),
		LastPartialSuccessAt: time.Date(2026, 11, 20, 22, 31, 0, 0, time.UTC),
		LastError:            "",
		FailedCommodities:    []string{"NIFTY"},
		LastDurationMS:       31000,
		LastFull:             false,
		LastJobID:            7,
		LastJobSuccess:       false,
		LastJobPartial:       true,
	}
	require.NoError(t, store.Save(want))

	got, err := store.Load()
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestFileStateStoreMissingFileIsZeroState(t *testing.T) {
	store := NewFileStateStore(filepath.Join(t.TempDir(), "missing.json"))
	got, err := store.Load()
	require.NoError(t, err)
	assert.Equal(t, State{}, got)
}

func TestFileStateStoreCorruptFileReturnsZeroAndError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	require.NoError(t, os.WriteFile(path, []byte("{not-json"), 0o644))

	got, err := NewFileStateStore(path).Load()
	require.Error(t, err)
	assert.Equal(t, State{}, got)
}

func TestStatePathForDB(t *testing.T) {
	assert.Equal(t, filepath.Join("tmp", "paisa.db.price-update.json"), StatePathForDB(filepath.Join("tmp", "paisa.db")))
}

func TestFileStateStoreWritesDocumentedFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	store := NewFileStateStore(path)

	require.NoError(t, store.Save(State{
		LastAttemptAt:     time.Date(2026, 11, 20, 22, 30, 0, 0, time.UTC),
		FailedCommodities: []string{"NIFTY"},
	}))

	raw, err := os.ReadFile(path)
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(raw, &got))

	assert.Equal(t, float64(1), got["schema_version"])
	assert.Equal(t, "2026-11-20T22:30:00Z", got["last_attempt_at"])
	assert.Equal(t, "", got["last_success_at"])
	assert.Equal(t, "", got["last_partial_success_at"])
	assert.Equal(t, "", got["last_error"])
	assert.Equal(t, "", got["last_warning"])
	assert.Equal(t, []any{"NIFTY"}, got["failed_commodities"])
	assert.Equal(t, float64(0), got["last_duration_ms"])
	assert.Equal(t, false, got["last_full"])
	assert.Equal(t, float64(0), got["last_job_id"])
	assert.Equal(t, false, got["last_job_success"])
	assert.Equal(t, false, got["last_job_partial"])
}

func TestFileStateStoreSaveReplacesFileAndLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	store := NewFileStateStore(path)

	require.NoError(t, store.Save(State{LastJobID: 1}))
	require.NoError(t, store.Save(State{LastJobID: 2}))

	got, err := store.Load()
	require.NoError(t, err)
	assert.Equal(t, uint64(2), got.LastJobID)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "state.json", entries[0].Name())
}

func TestFileStateStoreLoadNormalizesSchemaVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"last_job_id": 5}`), 0o644))

	got, err := NewFileStateStore(path).Load()
	require.NoError(t, err)
	assert.Equal(t, 1, got.SchemaVersion)
	assert.Equal(t, uint64(5), got.LastJobID)
}

func TestMemoryStateStoreRoundTrip(t *testing.T) {
	store := NewMemoryStateStore()

	want := State{
		SchemaVersion:     1,
		LastAttemptAt:     time.Date(2026, 11, 20, 22, 30, 0, 0, time.UTC),
		FailedCommodities: []string{"NIFTY"},
		LastJobID:         7,
	}
	require.NoError(t, store.Save(want))

	got, err := store.Load()
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestMemoryStateStoreReturnsCopies(t *testing.T) {
	store := NewMemoryStateStore()
	require.NoError(t, store.Save(State{FailedCommodities: []string{"NIFTY"}}))

	got, err := store.Load()
	require.NoError(t, err)
	got.FailedCommodities[0] = "CHANGED"

	again, err := store.Load()
	require.NoError(t, err)
	assert.Equal(t, []string{"NIFTY"}, again.FailedCommodities)
}

func TestMemoryStateStoreFreshIsZeroState(t *testing.T) {
	got, err := NewMemoryStateStore().Load()
	require.NoError(t, err)
	assert.Equal(t, State{}, got)
}
