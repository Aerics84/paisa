package pricesync

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ananthakumaran/paisa/internal/config"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testConfig(enabled bool) config.Config {
	cfg := config.Config{
		TimeZone: "Europe/Berlin",
		Commodities: []config.Commodity{{
			Name: "AAPL", Type: config.Stock,
			Price: config.Price{Provider: "com-yahoo", Code: "AAPL"},
		}},
	}
	if !enabled {
		cfg.PriceUpdate = &config.PriceUpdateConfig{Enabled: config.No}
	}
	return cfg
}

func newTestManager(store StateStore, now time.Time, cfg func() config.Config, syncFn func(SyncOptions) Outcome) *Manager {
	return New(nil, cfg, store, syncFn, NewRunner(), func() time.Time { return now })
}

func TestManagerCachesInvalidScheduleWarnings(t *testing.T) {
	var logs bytes.Buffer
	previousOutput := log.StandardLogger().Out
	log.SetOutput(&logs)
	t.Cleanup(func() {
		log.SetOutput(previousOutput)
	})

	cfg := testConfig(true)
	cfg.PriceUpdate = &config.PriceUpdateConfig{
		Enabled: config.Yes,
		Time:    "25:00",
		Days:    []string{"funday"},
	}
	now := time.Date(2026, 11, 23, 22, 45, 0, 0, berlin(t))
	m := newTestManager(NewMemoryStateStore(), now, func() config.Config { return cfg },
		func(SyncOptions) Outcome { return Outcome{Success: true} })

	for i := 0; i < 5; i++ {
		m.Status(now)
	}

	assert.Equal(t, 1, strings.Count(logs.String(), "Unknown price update day"))
}

func TestManagerStartIsAsyncAndAssignsJobID(t *testing.T) {
	store := NewMemoryStateStore()
	now := time.Date(2026, 11, 23, 22, 45, 0, 0, berlin(t))

	start := make(chan struct{})
	release := make(chan struct{})
	syncFn := func(SyncOptions) Outcome {
		close(start)
		<-release
		return Outcome{Success: true}
	}

	m := newTestManager(store, now, func() config.Config { return testConfig(true) }, syncFn)

	status, started := m.Start(SyncOptions{Full: false}, StartManual)
	require.True(t, started)
	assert.True(t, status.Running)
	assert.Equal(t, uint64(1), status.JobID)

	<-start
	close(release)

	require.Eventually(t, func() bool {
		return !m.Status(now).Running
	}, time.Second, 10*time.Millisecond)

	status = m.Status(now)
	assert.Equal(t, uint64(1), status.LastJobID)
	assert.True(t, status.LastJobSuccess)
	assert.NotNil(t, status.LastSuccessAt)
}

func TestManagerSecondStartDoesNotStartAnotherJob(t *testing.T) {
	store := NewMemoryStateStore()
	now := time.Date(2026, 11, 23, 22, 45, 0, 0, berlin(t))

	start := make(chan struct{})
	release := make(chan struct{})
	var calls int32
	syncFn := func(SyncOptions) Outcome {
		atomic.AddInt32(&calls, 1)
		close(start)
		<-release
		return Outcome{Success: true}
	}

	m := newTestManager(store, now, func() config.Config { return testConfig(true) }, syncFn)

	first, started := m.Start(SyncOptions{}, StartManual)
	require.True(t, started)
	<-start

	second, started := m.Start(SyncOptions{}, StartManual)
	require.False(t, started)
	assert.True(t, second.Running)
	assert.Equal(t, first.JobID, second.JobID)

	close(release)
	require.Eventually(t, func() bool {
		return !m.Status(now).Running
	}, time.Second, 10*time.Millisecond)

	assert.Equal(t, int32(1), atomic.LoadInt32(&calls))
}

func TestManagerRecordsPartialFailure(t *testing.T) {
	store := NewMemoryStateStore()
	now := time.Date(2026, 11, 23, 22, 45, 0, 0, berlin(t))

	syncFn := func(SyncOptions) Outcome {
		return Outcome{Success: false, Partial: true, FailedCommodities: []string{"NFLX"}, Error: "boom"}
	}

	m := newTestManager(store, now, func() config.Config { return testConfig(true) }, syncFn)

	_, started := m.Start(SyncOptions{}, StartManual)
	require.True(t, started)

	require.Eventually(t, func() bool {
		return !m.Status(now).Running
	}, time.Second, 10*time.Millisecond)

	status := m.Status(now)
	assert.NotNil(t, status.LastPartialSuccessAt)
	assert.Nil(t, status.LastSuccessAt)
	assert.True(t, status.LastJobPartial)
	assert.Equal(t, []string{"NFLX"}, status.FailedCommodities)
	assert.Equal(t, "boom", status.LastError)

	status.FailedCommodities[0] = "MUTATED"
	assert.Equal(t, []string{"NFLX"}, m.Status(now).FailedCommodities)

	state, err := store.Load()
	require.NoError(t, err)
	assert.True(t, state.LastSuccessAt.IsZero())
	assert.False(t, state.LastPartialSuccessAt.IsZero())
	assert.Equal(t, []string{"NFLX"}, state.FailedCommodities)
	assert.False(t, state.LastJobSuccess)
	assert.True(t, state.LastJobPartial)
}

func TestManagerTotalFailureAfterPartialIsNotPartial(t *testing.T) {
	store := NewMemoryStateStore()
	now := time.Date(2026, 11, 23, 22, 45, 0, 0, berlin(t))

	// A previous partial run left its timestamp and partial flag behind.
	require.NoError(t, store.Save(State{
		LastSuccessAt:        now.Add(-2 * time.Hour),
		LastPartialSuccessAt: now.Add(-time.Hour),
		LastJobID:            3,
		LastJobPartial:       true,
	}))

	syncFn := func(SyncOptions) Outcome {
		return Outcome{
			Success:           false,
			Partial:           false,
			FailedCommodities: []string{"AAPL"},
			Error:             "outage",
		}
	}

	m := newTestManager(store, now, func() config.Config { return testConfig(true) }, syncFn)

	_, started := m.Start(SyncOptions{}, StartManual)
	require.True(t, started)
	require.Eventually(t, func() bool {
		return !m.Status(now).Running
	}, time.Second, 10*time.Millisecond)

	status := m.Status(now)
	assert.False(t, status.LastJobSuccess)
	assert.False(t, status.LastJobPartial)
	assert.Equal(t, "outage", status.LastError)
	assert.Equal(t, []string{"AAPL"}, status.FailedCommodities)

	state, err := store.Load()
	require.NoError(t, err)
	assert.False(t, state.LastJobPartial)
	assert.Equal(t, "outage", state.LastError)
}

func TestManagerCooldownAndManualOverride(t *testing.T) {
	store := NewMemoryStateStore()
	now := time.Date(2026, 11, 23, 22, 45, 0, 0, berlin(t))

	// The last attempt was 10 minutes ago (inside the 30 minute cooldown) and
	// the last success predates the most recent slot (Monday 22:30), so the
	// state is stale but still cooling down.
	require.NoError(t, store.Save(State{
		LastAttemptAt: now.Add(-10 * time.Minute),
		LastSuccessAt: now.Add(-2 * time.Hour),
		LastJobID:     5,
	}))

	syncFn := func(SyncOptions) Outcome { return Outcome{Success: true} }
	m := newTestManager(store, now, func() config.Config { return testConfig(true) }, syncFn)

	// Automatic start respects the cooldown and refuses to run.
	status, started := m.Start(SyncOptions{}, StartAutomatic)
	require.False(t, started)
	assert.False(t, status.ShouldUpdate)

	// Manual start ignores the cooldown and runs anyway.
	_, started = m.Start(SyncOptions{}, StartManual)
	require.True(t, started)
	require.Eventually(t, func() bool {
		return !m.Status(now).Running
	}, time.Second, 10*time.Millisecond)
}

func TestManagerShouldUpdateWhenUnmanagedOrReadonly(t *testing.T) {
	now := time.Date(2026, 11, 23, 22, 45, 0, 0, berlin(t))
	noop := func(SyncOptions) Outcome { return Outcome{} }

	// No commodities configured: nothing to manage.
	unmanagedCfg := func() config.Config { return config.Config{TimeZone: "Europe/Berlin"} }
	m := newTestManager(NewMemoryStateStore(), now, unmanagedCfg, noop)
	status := m.Status(now)
	assert.False(t, status.Managed)
	assert.False(t, status.ShouldUpdate)

	// Readonly configuration: disabled even though a commodity is managed.
	readonlyCfg := func() config.Config {
		cfg := testConfig(true)
		cfg.Readonly = true
		return cfg
	}
	m = newTestManager(NewMemoryStateStore(), now, readonlyCfg, noop)
	status = m.Status(now)
	assert.True(t, status.Managed)
	assert.False(t, status.Enabled)
	assert.False(t, status.ShouldUpdate)
}

func TestManagerSchedulerCatchesUpAndStops(t *testing.T) {
	store := NewMemoryStateStore()
	now := time.Date(2026, 11, 23, 22, 45, 0, 0, berlin(t))

	var calls int32
	syncFn := func(SyncOptions) Outcome {
		atomic.AddInt32(&calls, 1)
		return Outcome{Success: true}
	}

	m := newTestManager(store, now, func() config.Config { return testConfig(true) }, syncFn)

	ctx, cancel := context.WithCancel(context.Background())
	m.StartScheduler(ctx)

	// The state has never succeeded, so the scheduler catches up on the missed
	// Monday slot and starts exactly one job.
	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&calls) >= 1
	}, time.Second, 10*time.Millisecond)

	cancel()

	require.Eventually(t, func() bool {
		return !m.Status(now).Running
	}, time.Second, 10*time.Millisecond)

	// Once stopped the scheduler must not launch further jobs.
	callsAfterCancel := atomic.LoadInt32(&calls)
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, callsAfterCancel, atomic.LoadInt32(&calls))
}

func TestManagerWakeRecomputesNextRun(t *testing.T) {
	store := NewMemoryStateStore()
	now := time.Date(2026, 11, 23, 22, 45, 0, 0, berlin(t))

	// The last success is recent enough to satisfy the Sunday-only schedule
	// (whose most recent slot was Sunday 22:30) but predates the Monday 22:30
	// slot of the default schedule.
	require.NoError(t, store.Save(State{LastSuccessAt: now.Add(-2 * time.Hour)}))

	var cfgMu sync.Mutex
	current := testConfig(true)
	current.PriceUpdate = &config.PriceUpdateConfig{Enabled: config.Yes, Time: "22:30", Days: []string{"sun"}}
	cfgFn := func() config.Config {
		cfgMu.Lock()
		defer cfgMu.Unlock()
		return current
	}

	var calls int32
	syncFn := func(SyncOptions) Outcome {
		atomic.AddInt32(&calls, 1)
		return Outcome{Success: true}
	}

	m := newTestManager(store, now, cfgFn, syncFn)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.StartScheduler(ctx)

	// The next slot is next Sunday, so nothing should run yet.
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, int32(0), atomic.LoadInt32(&calls))

	// Reconfigure to the default Mon-Fri schedule whose Monday 22:30 slot has
	// already passed, making the state stale, then wake the scheduler.
	cfgMu.Lock()
	current = testConfig(true)
	cfgMu.Unlock()

	m.Wake()

	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&calls) >= 1
	}, time.Second, 10*time.Millisecond)
}

func TestManagerSchedulerDoesNotStartJobAfterCancellation(t *testing.T) {
	store := NewMemoryStateStore()
	now := time.Date(2026, 11, 23, 22, 45, 0, 0, berlin(t))

	var calls int32
	syncFn := func(SyncOptions) Outcome {
		atomic.AddInt32(&calls, 1)
		return Outcome{Success: true}
	}

	m := newTestManager(store, now, func() config.Config { return testConfig(true) }, syncFn)

	// Cancel before the scheduler starts: a cancelled scheduler must never
	// launch a catch-up job, even though the state is stale.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	m.StartScheduler(ctx)

	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, int32(0), atomic.LoadInt32(&calls))

	// A wake after cancellation must not start a job either.
	m.Wake()
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, int32(0), atomic.LoadInt32(&calls))
}

func TestManagerSuccessClearsPartialSuccess(t *testing.T) {
	store := NewMemoryStateStore()
	now := time.Date(2026, 11, 23, 22, 45, 0, 0, berlin(t))

	// Simulate a previous partial run.
	require.NoError(t, store.Save(State{
		LastSuccessAt:        now.Add(-2 * time.Hour),
		LastPartialSuccessAt: now.Add(-time.Hour),
		LastJobID:            3,
	}))

	syncFn := func(SyncOptions) Outcome { return Outcome{Success: true} }
	m := newTestManager(store, now, func() config.Config { return testConfig(true) }, syncFn)

	_, started := m.Start(SyncOptions{}, StartManual)
	require.True(t, started)
	require.Eventually(t, func() bool {
		return !m.Status(now).Running
	}, time.Second, 10*time.Millisecond)

	state, err := store.Load()
	require.NoError(t, err)
	assert.False(t, state.LastSuccessAt.IsZero())
	assert.True(t, state.LastPartialSuccessAt.IsZero())
}

func TestManagerStatusReportsZeroJobIDForForeignRun(t *testing.T) {
	store := NewMemoryStateStore()
	now := time.Date(2026, 11, 23, 22, 45, 0, 0, berlin(t))

	// A previous manager job finished successfully.
	require.NoError(t, store.Save(State{LastJobID: 7, LastJobSuccess: true}))

	runner := NewRunner()
	m := New(nil, func() config.Config { return testConfig(true) }, store,
		func(SyncOptions) Outcome { return Outcome{Success: true} }, runner,
		func() time.Time { return now })

	// Idle: no job is reported.
	status := m.Status(now)
	assert.False(t, status.Running)
	assert.Equal(t, uint64(0), status.JobID)
	assert.Equal(t, uint64(7), status.LastJobID)

	// A foreign/foreground sync holds the runner without a manager job: the
	// status must report running with a zero job id, keeping last_job_id.
	require.True(t, runner.TryStart())
	status = m.Status(now)
	assert.True(t, status.Running)
	assert.Equal(t, uint64(0), status.JobID)
	assert.Equal(t, uint64(7), status.LastJobID)

	// Releasing the runner returns to the idle status.
	runner.Finish()
	status = m.Status(now)
	assert.False(t, status.Running)
	assert.Equal(t, uint64(0), status.JobID)
	assert.Equal(t, uint64(7), status.LastJobID)
}
