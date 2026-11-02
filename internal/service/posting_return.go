package service

import (
	"math"
	"time"

	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/ananthakumaran/paisa/internal/utils"
	"github.com/shopspring/decimal"
)

// PostingReturn is the unrealized return of a single commodity posting (one lot).
type PostingReturn struct {
	// PostingID identifies the source posting, so consumers can associate a
	// return with its lot instead of relying on array positions.
	PostingID      uint            `json:"posting_id"`
	UnrealizedGain decimal.Decimal `json:"unrealized_gain"`
	HoldingDays    int             `json:"holding_days"`
	// AnnualizedReturn is a compound annual rate in percent (the same scale as
	// XIRR). It is nil for lots held for less than a year: such a period is
	// reported as an absolute gain, never extrapolated.
	AnnualizedReturn *decimal.Decimal `json:"annualized_return"`
}

// ComputePostingReturn returns the return of a lot as of today. It reports false
// for postings that are not an open lot: currency postings, sales, stock splits
// and lots bought today.
func ComputePostingReturn(p posting.Posting, today time.Time) (PostingReturn, bool) {
	if utils.IsCurrency(p.Commodity) || !p.Quantity.IsPositive() || !p.Amount.IsPositive() {
		return PostingReturn{}, false
	}

	days := calendarDayOrdinal(today) - calendarDayOrdinal(p.Date)
	if days <= 0 {
		return PostingReturn{}, false
	}

	result := PostingReturn{UnrealizedGain: p.MarketAmount.Sub(p.Amount), HoldingDays: days}
	if days >= 365 && !p.MarketAmount.IsNegative() {
		ratio, _ := p.MarketAmount.Div(p.Amount).Float64()
		annualized := decimal.NewFromFloat((math.Pow(ratio, 365/float64(days)) - 1) * 100).Round(2)
		result.AnnualizedReturn = &annualized
	}
	return result, true
}

// calendarDayOrdinal returns the number of whole calendar days between the
// given time and the Unix epoch, ignoring the time of day and the location.
// The reference implementation used a calendarOrdinal helper from its utils
// package; this fork does not have one, so the day difference is computed
// here instead.
func calendarDayOrdinal(t time.Time) int {
	year, month, day := t.Date()
	utc := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	return int(utc.Unix() / (24 * 60 * 60))
}
