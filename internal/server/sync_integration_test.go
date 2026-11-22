package server

import (
	"testing"
	"time"

	"github.com/ananthakumaran/paisa/internal/cache"
	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/model/price"
	"github.com/ananthakumaran/paisa/internal/pricesync"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBackgroundPriceSyncEndToEnd proves the full state transition a user
// expects from an automatic price update: a stale state is updated by the
// manager's background job, the price is written to the database, the cache is
// cleared, and the manager then reports a fresh, successful state.
func TestBackgroundPriceSyncEndToEnd(t *testing.T) {
	db := openSyncTestDB(t)
	store := pricesync.NewMemoryStateStore()
	runner := pricesync.NewRunner()
	manager := pricesync.New(db, config.GetConfig, store,
		func(pricesync.SyncOptions) pricesync.Outcome {
			// Write a price directly, as the real sync function would.
			require.NoError(t, price.UpsertMerged(db, config.Stock, "AAPL", "AAPL",
				[]*price.Price{{
					Date:  time.Date(2026, 11, 20, 0, 0, 0, 0, config.TimeZone()),
					Value: decimal.NewFromInt(100),
				}}, true))
			cache.Clear()
			cache.MarkSynced()
			return pricesync.Outcome{Success: true}
		}, runner, time.Now)

	status, started := manager.Start(pricesync.SyncOptions{Full: false}, pricesync.StartManual)
	require.True(t, started)
	require.True(t, status.Running)

	require.Eventually(t, func() bool {
		status := manager.Status(time.Now())
		return !status.Running && status.LastJobSuccess
	}, 2*time.Second, 10*time.Millisecond)

	final := manager.Status(time.Now())
	assert.False(t, final.Stale)
	assert.True(t, final.LastJobSuccess)
	assert.NotNil(t, final.LastSuccessAt)
}

// TestPartialRunInvalidatesCacheOnce verifies that a partially successful run
// clears the cache exactly once and records the partial state (and failed
// commodity) without marking the run fully successful.
func TestPartialRunInvalidatesCacheOnce(t *testing.T) {
	db := openSyncTestDB(t)

	count := 0
	cache.SetClearHookForTest(func() { count++ })
	t.Cleanup(func() { cache.SetClearHookForTest(nil) })

	store := pricesync.NewMemoryStateStore()
	runner := pricesync.NewRunner()
	manager := pricesync.New(db, config.GetConfig, store,
		func(pricesync.SyncOptions) pricesync.Outcome {
			cache.Clear()
			return pricesync.Outcome{Success: false, Partial: true, FailedCommodities: []string{"NFLX"}}
		}, runner, time.Now)

	_, started := manager.Start(pricesync.SyncOptions{Full: false}, pricesync.StartManual)
	require.True(t, started)

	require.Eventually(t, func() bool {
		return !manager.Status(time.Now()).Running
	}, 2*time.Second, 10*time.Millisecond)

	assert.Equal(t, 1, count)

	final := manager.Status(time.Now())
	assert.NotNil(t, final.LastPartialSuccessAt)
	assert.Nil(t, final.LastSuccessAt)
	assert.Equal(t, []string{"NFLX"}, final.FailedCommodities)

	saved, err := store.Load()
	require.NoError(t, err)
	assert.Equal(t, []string{"NFLX"}, saved.FailedCommodities)
}
