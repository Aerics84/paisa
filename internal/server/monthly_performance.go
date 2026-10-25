package server

import (
	"sort"
	"time"

	"github.com/ananthakumaran/paisa/internal/model/transaction"
	"github.com/ananthakumaran/paisa/internal/utils"
	"github.com/shopspring/decimal"
)

const (
	PositionCashFlowPurchase = "purchase"
	PositionCashFlowSale     = "sale"
	PositionCashFlowDividend = "dividend"

	// DefaultMaxPriceAgeDays tolerates a long weekend plus a market holiday
	// without accepting arbitrarily old carried-forward quotes.
	DefaultMaxPriceAgeDays = 7
)

// PositionValuation is the base-currency market value of a position on one
// calendar day together with the date of the quote that produced it.
type PositionValuation struct {
	Date        time.Time       `json:"date"`
	MarketValue decimal.Decimal `json:"market_value"`
	PriceDate   time.Time       `json:"price_date"`
}

// PositionCashFlow is an external movement of value into or out of a position.
// Dividends are kept separate because they contribute to total return without
// changing the position's market value.
type PositionCashFlow struct {
	Date   time.Time       `json:"date"`
	Kind   string          `json:"kind"`
	Amount decimal.Decimal `json:"amount"`
}

// PositionMonthlyInput is everything needed to compute a flow-adjusted total
// return for one position in one calendar period.
type PositionMonthlyInput struct {
	Account         string              `json:"account"`
	PeriodStart     time.Time           `json:"period_start"`
	PeriodEnd       time.Time           `json:"period_end"`
	DailyValuations []PositionValuation `json:"daily_valuations"`
	CashFlows       []PositionCashFlow  `json:"cash_flows"`
	// MaxPriceAgeDays is the largest accepted distance between a valuation day
	// and the quote date it carries forward. Zero uses the default.
	MaxPriceAgeDays int `json:"max_price_age_days"`
}

// MonthlyPositionPerformance is the selected-period result for one position.
// TotalReturnPercent is a time-weighted return; nil-equivalent results are
// expressed by the bool returned from calculateMonthlyPositionPerformance.
type MonthlyPositionPerformance struct {
	Account            string          `json:"account"`
	GainLoss           decimal.Decimal `json:"gain_loss"`
	TotalReturnPercent decimal.Decimal `json:"total_return_percent"`
	DividendIncome     decimal.Decimal `json:"dividend_income"`
	NetContributions   decimal.Decimal `json:"net_contributions"`
	NetWithdrawals     decimal.Decimal `json:"net_withdrawals"`
	PriceAsOf          time.Time       `json:"price_as_of"`
}

func normalizePerformanceDate(date time.Time) time.Time {
	return time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
}

// isQuantityOnlyTransaction reports a corporate action that changes a
// position's units without any cash leg, such as a stock split or a
// consolidation. Market values alone cannot distinguish that unit change from
// performance, so callers exclude the position instead of ranking a distorted
// return.
func isQuantityOnlyTransaction(t transaction.Transaction) bool {
	if len(t.Postings) < 2 {
		return false
	}

	account := t.Postings[0].Account
	for _, p := range t.Postings {
		if utils.IsCurrency(p.Commodity) || p.Account != account {
			return false
		}
	}

	return true
}

func performanceDaysBetween(earlier time.Time, later time.Time) int {
	return int(normalizePerformanceDate(later).Sub(normalizePerformanceDate(earlier)).Hours() / 24)
}

func positionFlowsByDate(flows []PositionCashFlow) (map[time.Time][3]decimal.Decimal, bool) {
	byDate := make(map[time.Time][3]decimal.Decimal)

	for _, flow := range flows {
		if flow.Amount.LessThan(decimal.Zero) {
			return nil, false
		}

		var bucket int
		switch flow.Kind {
		case PositionCashFlowPurchase:
			bucket = 0
		case PositionCashFlowSale:
			bucket = 1
		case PositionCashFlowDividend:
			bucket = 2
		default:
			return nil, false
		}

		key := normalizePerformanceDate(flow.Date)
		entry := byDate[key]
		entry[bucket] = entry[bucket].Add(flow.Amount)
		byDate[key] = entry
	}

	return byDate, true
}

// calculateMonthlyPositionPerformance computes a daily-chained time-weighted
// total return. Purchases and sales are cash flows and never count as return;
// dividends are added to the return and reported separately. The function
// returns false whenever the valuation or flow coverage cannot support a
// trustworthy number, so callers never present a missing result as 0%.
func calculateMonthlyPositionPerformance(input PositionMonthlyInput) (MonthlyPositionPerformance, bool) {
	if len(input.DailyValuations) < 2 {
		return MonthlyPositionPerformance{}, false
	}

	maxPriceAge := input.MaxPriceAgeDays
	if maxPriceAge <= 0 {
		maxPriceAge = DefaultMaxPriceAgeDays
	}

	valuations := append([]PositionValuation(nil), input.DailyValuations...)
	sort.Slice(valuations, func(i, j int) bool { return valuations[i].Date.Before(valuations[j].Date) })

	deduplicated := make([]PositionValuation, 0, len(valuations))
	for _, valuation := range valuations {
		if len(deduplicated) > 0 && normalizePerformanceDate(deduplicated[len(deduplicated)-1].Date).Equal(normalizePerformanceDate(valuation.Date)) {
			deduplicated[len(deduplicated)-1] = valuation
			continue
		}
		deduplicated = append(deduplicated, valuation)
	}
	valuations = deduplicated

	if !normalizePerformanceDate(valuations[0].Date).Equal(normalizePerformanceDate(input.PeriodStart)) {
		return MonthlyPositionPerformance{}, false
	}
	if !normalizePerformanceDate(valuations[len(valuations)-1].Date).Equal(normalizePerformanceDate(input.PeriodEnd)) {
		return MonthlyPositionPerformance{}, false
	}

	flowsByDate, ok := positionFlowsByDate(input.CashFlows)
	if !ok {
		return MonthlyPositionPerformance{}, false
	}

	quoteDates := make(map[time.Time]bool)
	valuationDates := make(map[time.Time]bool)
	var priceAsOf time.Time
	for _, valuation := range valuations {
		valuationDate := normalizePerformanceDate(valuation.Date)
		valuationDates[valuationDate] = true

		flows := flowsByDate[valuationDate]
		hasFlow := !flows[0].IsZero() || !flows[1].IsZero() || !flows[2].IsZero()
		// A day without holdings and without a flow has no capital at risk;
		// it needs no quote and must not inflate the coverage count.
		if !valuation.MarketValue.GreaterThan(decimal.Zero) && !hasFlow {
			continue
		}

		if valuation.PriceDate.IsZero() {
			return MonthlyPositionPerformance{}, false
		}

		priceDate := normalizePerformanceDate(valuation.PriceDate)
		if priceDate.After(valuationDate) {
			return MonthlyPositionPerformance{}, false
		}
		if performanceDaysBetween(priceDate, valuationDate) > maxPriceAge {
			return MonthlyPositionPerformance{}, false
		}

		quoteDates[priceDate] = true
		priceAsOf = valuation.PriceDate
	}

	// A single carried-forward quote for the whole period would turn a lack of
	// price observations into a silent 0% return.
	if len(quoteDates) < 2 {
		return MonthlyPositionPerformance{}, false
	}

	for flowDate := range flowsByDate {
		if !valuationDates[flowDate] {
			return MonthlyPositionPerformance{}, false
		}
	}

	netContributions := decimal.Zero
	netWithdrawals := decimal.Zero
	dividendIncome := decimal.Zero
	for _, flow := range input.CashFlows {
		switch flow.Kind {
		case PositionCashFlowPurchase:
			netContributions = netContributions.Add(flow.Amount)
		case PositionCashFlowSale:
			netWithdrawals = netWithdrawals.Add(flow.Amount)
		case PositionCashFlowDividend:
			dividendIncome = dividendIncome.Add(flow.Amount)
		}
	}

	growth := decimal.NewFromInt(1)
	computedReturn := false
	previous := valuations[0]

	for _, valuation := range valuations[1:] {
		entry := flowsByDate[normalizePerformanceDate(valuation.Date)]
		purchased, sold, dividends := entry[0], entry[1], entry[2]

		if previous.MarketValue.GreaterThan(decimal.Zero) {
			// Value left at the end of the subperiod, excluding external
			// purchases, plus sale proceeds and dividends. The result is what
			// the capital at the start of the subperiod actually earned.
			numerator := valuation.MarketValue.Sub(purchased).Add(sold).Add(dividends)
			if numerator.LessThan(decimal.Zero) {
				return MonthlyPositionPerformance{}, false
			}

			growth = growth.Mul(numerator.Div(previous.MarketValue))
			computedReturn = true
		}

		previous = valuation
	}

	if !computedReturn {
		return MonthlyPositionPerformance{}, false
	}

	startValue := valuations[0].MarketValue
	endValue := valuations[len(valuations)-1].MarketValue
	gainLoss := endValue.Sub(startValue).Sub(netContributions).Add(netWithdrawals).Add(dividendIncome)

	return MonthlyPositionPerformance{
		Account:            input.Account,
		GainLoss:           gainLoss,
		TotalReturnPercent: growth.Sub(decimal.NewFromInt(1)).Mul(decimal.NewFromInt(100)).Round(4),
		DividendIncome:     dividendIncome,
		NetContributions:   netContributions,
		NetWithdrawals:     netWithdrawals,
		PriceAsOf:          priceAsOf,
	}, true
}
