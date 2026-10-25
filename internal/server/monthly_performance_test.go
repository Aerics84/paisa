package server

import (
	"testing"
	"time"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/ananthakumaran/paisa/internal/model/transaction"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func loadMonthlyReviewConfig(t *testing.T) {
	t.Helper()

	err := config.LoadConfig([]byte(`
journal_path: main.ledger
db_path: paisa.db
default_currency: EUR
time_zone: UTC
`), "")
	require.NoError(t, err)
}

func performanceTestDate(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func performanceTestValuation(date time.Time, value string, priceDate time.Time) PositionValuation {
	return PositionValuation{
		Date:        date,
		MarketValue: decimal.RequireFromString(value),
		PriceDate:   priceDate,
	}
}

func performanceTestFlow(date time.Time, kind string, amount string) PositionCashFlow {
	return PositionCashFlow{
		Date:   date,
		Kind:   kind,
		Amount: decimal.RequireFromString(amount),
	}
}

func requireDecimalEqual(t *testing.T, expected string, actual decimal.Decimal, msgAndArgs ...interface{}) {
	t.Helper()
	require.True(t, actual.Equal(decimal.RequireFromString(expected)), append([]interface{}{"expected " + expected + ", got " + actual.String()}, msgAndArgs...)...)
}

func TestMonthlyPerformanceKeepsBuysAndSellsOutOfReturn(t *testing.T) {
	start := performanceTestDate(2022, time.January, 1)
	mid := performanceTestDate(2022, time.January, 15)
	end := performanceTestDate(2022, time.January, 31)

	t.Run("mid month purchase earns only from the purchase date", func(t *testing.T) {
		performance, ok := calculateMonthlyPositionPerformance(PositionMonthlyInput{
			Account:     "Assets:Equity:VWCE",
			PeriodStart: start,
			PeriodEnd:   end,
			DailyValuations: []PositionValuation{
				performanceTestValuation(start, "0", start),
				performanceTestValuation(mid, "100", mid),
				performanceTestValuation(end, "110", end),
			},
			CashFlows: []PositionCashFlow{performanceTestFlow(mid, PositionCashFlowPurchase, "100")},
		})

		require.True(t, ok)
		requireDecimalEqual(t, "10", performance.TotalReturnPercent)
		requireDecimalEqual(t, "10", performance.GainLoss)
		requireDecimalEqual(t, "100", performance.NetContributions)
		requireDecimalEqual(t, "0", performance.NetWithdrawals)
	})

	t.Run("partial sale is a withdrawal and not a loss", func(t *testing.T) {
		performance, ok := calculateMonthlyPositionPerformance(PositionMonthlyInput{
			Account:     "Assets:Equity:VWCE",
			PeriodStart: start,
			PeriodEnd:   end,
			DailyValuations: []PositionValuation{
				performanceTestValuation(start, "1000", start),
				performanceTestValuation(mid, "800", mid),
				performanceTestValuation(end, "960", end),
			},
			CashFlows: []PositionCashFlow{performanceTestFlow(mid, PositionCashFlowSale, "200")},
		})

		require.True(t, ok)
		requireDecimalEqual(t, "20", performance.TotalReturnPercent)
		requireDecimalEqual(t, "160", performance.GainLoss)
		requireDecimalEqual(t, "200", performance.NetWithdrawals)
	})
}

func TestMonthlyPerformanceIncludesDividendsInTotalReturn(t *testing.T) {
	start := performanceTestDate(2022, time.January, 1)
	mid := performanceTestDate(2022, time.January, 15)
	end := performanceTestDate(2022, time.January, 31)

	performance, ok := calculateMonthlyPositionPerformance(PositionMonthlyInput{
		Account:     "Assets:Equity:VWCE",
		PeriodStart: start,
		PeriodEnd:   end,
		DailyValuations: []PositionValuation{
			performanceTestValuation(start, "1000", start),
			performanceTestValuation(mid, "1000", mid),
			performanceTestValuation(end, "1050", end),
		},
		CashFlows: []PositionCashFlow{performanceTestFlow(mid, PositionCashFlowDividend, "20")},
	})

	require.True(t, ok)
	requireDecimalEqual(t, "7.1", performance.TotalReturnPercent)
	requireDecimalEqual(t, "70", performance.GainLoss)
	requireDecimalEqual(t, "20", performance.DividendIncome)
	requireDecimalEqual(t, "0", performance.NetContributions)
	requireDecimalEqual(t, "0", performance.NetWithdrawals)
}

func TestMonthlyPerformanceHandlesFullyClosedPosition(t *testing.T) {
	start := performanceTestDate(2022, time.January, 1)
	mid := performanceTestDate(2022, time.January, 15)
	end := performanceTestDate(2022, time.January, 31)

	performance, ok := calculateMonthlyPositionPerformance(PositionMonthlyInput{
		Account:     "Assets:Equity:VWCE",
		PeriodStart: start,
		PeriodEnd:   end,
		DailyValuations: []PositionValuation{
			performanceTestValuation(start, "1000", start),
			performanceTestValuation(mid, "0", mid),
			performanceTestValuation(end, "0", end),
		},
		CashFlows: []PositionCashFlow{performanceTestFlow(mid, PositionCashFlowSale, "1100")},
	})

	require.True(t, ok)
	requireDecimalEqual(t, "10", performance.TotalReturnPercent)
	requireDecimalEqual(t, "100", performance.GainLoss)
	requireDecimalEqual(t, "1100", performance.NetWithdrawals)
	require.True(t, performance.PriceAsOf.Equal(mid))
}

func TestMonthlyPerformanceHandlesForeignCurrencyValues(t *testing.T) {
	start := performanceTestDate(2022, time.January, 1)
	end := performanceTestDate(2022, time.January, 31)

	// Values are already converted to the configured base currency, so both
	// asset and FX movement are part of the value series.
	performance, ok := calculateMonthlyPositionPerformance(PositionMonthlyInput{
		Account:     "Assets:Equity:US_STOCK",
		PeriodStart: start,
		PeriodEnd:   end,
		DailyValuations: []PositionValuation{
			performanceTestValuation(start, "1000", start),
			performanceTestValuation(end, "1100", end),
		},
	})

	require.True(t, ok)
	requireDecimalEqual(t, "10", performance.TotalReturnPercent)
	requireDecimalEqual(t, "100", performance.GainLoss)
}

func TestMonthlyPerformanceRejectsInsufficientPriceCoverage(t *testing.T) {
	start := performanceTestDate(2022, time.January, 1)
	mid := performanceTestDate(2022, time.January, 15)
	end := performanceTestDate(2022, time.January, 31)

	cases := []struct {
		name        string
		valuations []PositionValuation
		flows       []PositionCashFlow
	}{
		{
			name: "missing start quote",
			valuations: []PositionValuation{
				performanceTestValuation(start, "1000", time.Time{}),
				performanceTestValuation(end, "1100", end),
			},
		},
		{
			name: "missing end quote",
			valuations: []PositionValuation{
				performanceTestValuation(start, "1000", start),
				performanceTestValuation(end, "1100", time.Time{}),
			},
		},
		{
			name: "stale end quote",
			valuations: []PositionValuation{
				performanceTestValuation(start, "1000", start),
				performanceTestValuation(end, "1100", start),
			},
		},
		{
			name: "single quote date for the whole month",
			valuations: []PositionValuation{
				performanceTestValuation(start, "1000", start),
				performanceTestValuation(end, "1100", start),
			},
		},
		{
			name: "flow without a matching valuation day",
			valuations: []PositionValuation{
				performanceTestValuation(start, "1000", start),
				performanceTestValuation(end, "1100", end),
			},
			flows: []PositionCashFlow{performanceTestFlow(mid, PositionCashFlowPurchase, "100")},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, ok := calculateMonthlyPositionPerformance(PositionMonthlyInput{
				Account:         "Assets:Equity:VWCE",
				PeriodStart:     start,
				PeriodEnd:       end,
				DailyValuations: tc.valuations,
				CashFlows:       tc.flows,
			})
			require.False(t, ok, "missing or stale price coverage must not become a 0% return")
		})
	}
}

func TestMonthlyPerformanceAllowsWeekendCarryForward(t *testing.T) {
	friday := performanceTestDate(2022, time.January, 7)
	saturday := performanceTestDate(2022, time.January, 8)
	sunday := performanceTestDate(2022, time.January, 9)
	monday := performanceTestDate(2022, time.January, 10)

	performance, ok := calculateMonthlyPositionPerformance(PositionMonthlyInput{
		Account:     "Assets:Equity:VWCE",
		PeriodStart: friday,
		PeriodEnd:   monday,
		DailyValuations: []PositionValuation{
			performanceTestValuation(friday, "100", friday),
			performanceTestValuation(saturday, "100", friday),
			performanceTestValuation(sunday, "100", friday),
			performanceTestValuation(monday, "101", monday),
		},
	})

	require.True(t, ok, "a normal weekend carry-forward is not stale data")
	requireDecimalEqual(t, "1", performance.TotalReturnPercent)
}

func TestMonthlyPerformanceAcceptsCarryForwardWithinAgeLimit(t *testing.T) {
	start := performanceTestDate(2022, time.January, 1)
	end := performanceTestDate(2022, time.January, 31)
	lastQuote := performanceTestDate(2022, time.January, 29)

	performance, ok := calculateMonthlyPositionPerformance(PositionMonthlyInput{
		Account:     "Assets:Equity:VWCE",
		PeriodStart: start,
		PeriodEnd:   end,
		DailyValuations: []PositionValuation{
			performanceTestValuation(start, "100", start),
			performanceTestValuation(end, "110", lastQuote),
		},
		MaxPriceAgeDays: 7,
	})

	require.True(t, ok)
	requireDecimalEqual(t, "10", performance.TotalReturnPercent)
	require.True(t, performance.PriceAsOf.Equal(lastQuote))
}

func TestMonthlyPerformanceRejectsUnsupportedFlowKind(t *testing.T) {
	start := performanceTestDate(2022, time.January, 1)
	end := performanceTestDate(2022, time.January, 31)

	_, ok := calculateMonthlyPositionPerformance(PositionMonthlyInput{
		Account:     "Assets:Equity:VWCE",
		PeriodStart: start,
		PeriodEnd:   end,
		DailyValuations: []PositionValuation{
			performanceTestValuation(start, "100", start),
			performanceTestValuation(end, "110", end),
		},
		CashFlows: []PositionCashFlow{performanceTestFlow(start, "transfer", "100")},
	})

	require.False(t, ok, "an unknown flow role must not be guessed")
}

func TestMonthlyPerformanceDetectsQuantityOnlyStockSplit(t *testing.T) {
	loadMonthlyReviewConfig(t)

	split := transaction.Transaction{
		ID: "split-1",
		Postings: []posting.Posting{
			{Account: "Assets:Equity:VWCE", Commodity: "VWCE", Quantity: decimal.NewFromInt(10), Amount: decimal.Zero},
			{Account: "Assets:Equity:VWCE", Commodity: "VWCE", Quantity: decimal.NewFromInt(-1), Amount: decimal.Zero},
		},
	}
	require.True(t, isQuantityOnlyTransaction(split))

	buy := transaction.Transaction{
		ID: "buy-1",
		Postings: []posting.Posting{
			{Account: "Assets:Equity:VWCE", Commodity: "VWCE", Quantity: decimal.NewFromInt(10), Amount: decimal.NewFromInt(1000)},
			{Account: "Assets:Checking", Commodity: "EUR", Quantity: decimal.NewFromInt(-1000), Amount: decimal.NewFromInt(-1000)},
		},
	}
	require.False(t, isQuantityOnlyTransaction(buy), "a transaction with a cash leg is not a split")
}
