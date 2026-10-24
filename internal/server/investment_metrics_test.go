package server

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestCalculateSavingsMetricsReturnsSurplusAndOverallRate(t *testing.T) {
	metrics := calculateSavingsMetrics(decimal.NewFromInt(900), decimal.NewFromInt(600))

	require.True(t, metrics.Surplus.Equal(decimal.NewFromInt(300)), "surplus should be net income minus net expense")
	require.True(t, metrics.OverallRateAvailable, "overall rate should be available for positive net income")
	require.True(t, metrics.OverallRate.Sub(decimal.RequireFromString("33.3333")).Abs().LessThan(decimal.RequireFromString("0.001")),
		"overall rate should be 100 * surplus / net income, got %s", metrics.OverallRate)
}

func TestCalculateSavingsMetricsPreservesNegativeSurplus(t *testing.T) {
	metrics := calculateSavingsMetrics(decimal.NewFromInt(900), decimal.NewFromInt(1000))

	require.True(t, metrics.Surplus.Equal(decimal.NewFromInt(-100)), "negative surplus should not be clipped")
	require.True(t, metrics.OverallRateAvailable, "overall rate should be available for positive net income")
	require.True(t, metrics.OverallRate.LessThan(decimal.Zero), "overall rate should stay negative")
}

func TestCalculateSavingsMetricsMarksNonPositiveIncomeUnavailable(t *testing.T) {
	for _, netIncome := range []decimal.Decimal{decimal.Zero, decimal.NewFromInt(-100)} {
		metrics := calculateSavingsMetrics(netIncome, decimal.NewFromInt(600))

		require.False(t, metrics.OverallRateAvailable, "overall rate should be unavailable for net income %s", netIncome)
		require.True(t, metrics.OverallRate.Equal(decimal.Zero), "unavailable rate should stay zero, got %s", metrics.OverallRate)
		require.True(t, metrics.Surplus.Equal(netIncome.Sub(decimal.NewFromInt(600))), "surplus should be calculated regardless of availability")
	}
}
