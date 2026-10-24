package server

import "github.com/shopspring/decimal"

// SavingsMetrics describes the surplus of a period and the rate it
// represents relative to the net income of that period.
type SavingsMetrics struct {
	Surplus              decimal.Decimal
	OverallRate          decimal.Decimal
	OverallRateAvailable bool
}

// calculateSavingsMetrics derives the overall savings surplus and rate
// from net income and net expense. The rate is only meaningful for a
// positive net income; the surplus is always reported.
func calculateSavingsMetrics(netIncome, netExpense decimal.Decimal) SavingsMetrics {
	surplus := netIncome.Sub(netExpense)

	if netIncome.GreaterThan(decimal.Zero) {
		return SavingsMetrics{
			Surplus:              surplus,
			OverallRate:          surplus.Div(netIncome).Mul(decimal.NewFromInt(100)),
			OverallRateAvailable: true,
		}
	}

	return SavingsMetrics{
		Surplus:              surplus,
		OverallRate:          decimal.Zero,
		OverallRateAvailable: false,
	}
}
