package cmd

import (
	"testing"
	"time"

	"github.com/ananthakumaran/paisa/internal/pricesync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecordPriceUpdateStoresSuccess(t *testing.T) {
	store := pricesync.NewMemoryStateStore()
	started := time.Date(2026, 11, 20, 22, 30, 0, 0, time.UTC)
	finished := started.Add(30 * time.Second)

	err := recordPriceUpdate(store, started, finished, false, pricesync.Outcome{Success: true})
	require.NoError(t, err)
	got, err := store.Load()
	require.NoError(t, err)
	assert.Equal(t, started, got.LastAttemptAt)
	assert.Equal(t, finished, got.LastSuccessAt)
	assert.Equal(t, int64(30000), got.LastDurationMS)
	assert.False(t, got.LastFull)
	assert.True(t, got.LastJobSuccess)
}

func TestRecordPriceUpdateSuccessResetsTransientState(t *testing.T) {
	store := pricesync.NewMemoryStateStore()
	started := time.Date(2026, 11, 20, 22, 30, 0, 0, time.UTC)
	finished := started.Add(30 * time.Second)

	previousPartial := started.Add(-time.Hour)
	require.NoError(t, store.Save(pricesync.State{
		LastPartialSuccessAt: previousPartial,
		LastWarning:          "old warning",
		LastJobID:            42,
	}))

	err := recordPriceUpdate(store, started, finished, false, pricesync.Outcome{
		Success: true, Warning: "new warning",
	})
	require.NoError(t, err)
	got, err := store.Load()
	require.NoError(t, err)
	assert.True(t, got.LastPartialSuccessAt.IsZero())
	assert.Equal(t, uint64(42), got.LastJobID)
	assert.Equal(t, "new warning", got.LastWarning)
}

func TestRecordPriceUpdateStoresFailure(t *testing.T) {
	store := pricesync.NewMemoryStateStore()
	started := time.Date(2026, 11, 20, 22, 30, 0, 0, time.UTC)

	err := recordPriceUpdate(store, started, started.Add(time.Second), true, pricesync.Outcome{
		Success: false, Partial: true, FailedCommodities: []string{"NIFTY"}, Error: "boom",
	})
	require.NoError(t, err)
	got, _ := store.Load()
	assert.Equal(t, "boom", got.LastError)
	assert.Equal(t, []string{"NIFTY"}, got.FailedCommodities)
	assert.True(t, got.LastPartialSuccessAt.Equal(started.Add(time.Second)))
	assert.True(t, got.LastFull)
}
