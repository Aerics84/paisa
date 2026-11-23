package price

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func priceAt(day int, value string) *Price {
	return &Price{
		Date:  time.Date(2026, 11, day, 0, 0, 0, 0, time.UTC),
		Value: decimal.RequireFromString(value),
	}
}

func TestValidatePricesRejectsEmpty(t *testing.T) {
	_, err := ValidatePrices(nil, ValidateOptions{})
	require.Error(t, err)
}

func TestValidatePricesRejectsInvalidValuesAndFutureDates(t *testing.T) {
	now := time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)
	for _, p := range []*Price{
		{Date: now, Value: decimal.Zero},
		{Date: now, Value: decimal.NewFromInt(-1)},
		{Date: now.Add(72 * time.Hour), Value: decimal.NewFromInt(1)},
		{Date: time.Time{}, Value: decimal.NewFromInt(1)},
	} {
		_, err := ValidatePrices([]*Price{p}, ValidateOptions{Now: now})
		require.Error(t, err)
	}
}

func TestValidatePricesDeduplicatesByInstant(t *testing.T) {
	now := time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)
	got, err := ValidatePrices([]*Price{
		priceAt(20, "1"),
		priceAt(20, "2"),
		priceAt(21, "3"),
	}, ValidateOptions{Now: now})
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "2", got[0].Value.String())
	assert.Equal(t, "3", got[1].Value.String())
}

func TestValidatePricesRequiresOverlapWindow(t *testing.T) {
	now := time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)
	since := now.AddDate(0, 0, -7)
	_, err := ValidatePrices([]*Price{priceAt(10, "1")}, ValidateOptions{
		Since: since, Now: now, RequireSince: true,
	})
	require.Error(t, err)
}
