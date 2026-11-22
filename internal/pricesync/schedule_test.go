package pricesync

import (
	"testing"
	"time"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func berlin(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Berlin")
	require.NoError(t, err)
	return loc
}

func scheduleFixture(t *testing.T) Schedule {
	return ParseSchedule(config.DefaultPriceUpdateConfig(), berlin(t))
}

func TestMostRecentSlotOnWeekdays(t *testing.T) {
	s := scheduleFixture(t)
	loc := berlin(t)

	before, ok := s.MostRecentSlot(time.Date(2026, 11, 23, 22, 29, 0, 0, loc)) // Monday
	require.True(t, ok)
	assert.Equal(t, time.Date(2026, 11, 20, 22, 30, 0, 0, loc), before) // Friday

	after, ok := s.MostRecentSlot(time.Date(2026, 11, 23, 22, 31, 0, 0, loc))
	require.True(t, ok)
	assert.Equal(t, time.Date(2026, 11, 23, 22, 30, 0, 0, loc), after)
}

func TestMostRecentSlotOnWeekend(t *testing.T) {
	s := scheduleFixture(t)
	loc := berlin(t)

	got, ok := s.MostRecentSlot(time.Date(2026, 11, 22, 12, 0, 0, 0, loc)) // Sunday
	require.True(t, ok)
	assert.Equal(t, time.Date(2026, 11, 20, 22, 30, 0, 0, loc), got)
}

func TestNextSlotAcrossWeekend(t *testing.T) {
	s := scheduleFixture(t)
	loc := berlin(t)

	got, ok := s.NextSlot(time.Date(2026, 11, 20, 22, 31, 0, 0, loc)) // Friday after run
	require.True(t, ok)
	assert.Equal(t, time.Date(2026, 11, 23, 22, 30, 0, 0, loc), got) // Monday
}

func TestScheduleKeepsWallClockAcrossDST(t *testing.T) {
	s := scheduleFixture(t)
	loc := berlin(t)

	// DST ends 2026-10-25. The slot stays 22:30 local time.
	got, ok := s.NextSlot(time.Date(2026, 10, 24, 23, 0, 0, 0, loc))
	require.True(t, ok)
	assert.Equal(t, 22, got.Hour())
	assert.Equal(t, 30, got.Minute())
}

func TestInvalidScheduleFallsBackToDefaults(t *testing.T) {
	s := ParseSchedule(config.PriceUpdateConfig{
		Enabled: config.Yes,
		Time:    "not-a-time",
		Days:    []string{"funday"},
	}, berlin(t))
	assert.True(t, s.Active)
	assert.Equal(t, 22, s.Hour)
	assert.Equal(t, 30, s.Minute)
	assert.True(t, s.Weekdays[time.Monday])
	assert.False(t, s.Weekdays[time.Saturday])
}

func TestDisabledScheduleHasNoSlots(t *testing.T) {
	s := ParseSchedule(config.PriceUpdateConfig{Enabled: config.No}, berlin(t))
	assert.False(t, s.Active)
	_, ok := s.NextSlot(time.Now())
	assert.False(t, ok)
}
