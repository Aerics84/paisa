package goal

import (
	"github.com/ananthakumaran/paisa/internal/accounting"
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/ananthakumaran/paisa/internal/server/assets"
	"github.com/ananthakumaran/paisa/internal/service"
	"github.com/shopspring/decimal"
)

// RetirementGoalDetail is the JSON body returned by
// GET /api/goals/retirement/:name. Dates in the timeline are RFC3339 strings
// on the wire; the frontend client maps them to Dayjs.
type RetirementGoalDetail struct {
	Type                string                           `json:"type"`
	Name                string                           `json:"name"`
	Icon                string                           `json:"icon"`
	SavingsTimeline     []accounting.Point               `json:"savingsTimeline"`
	SavingsTotal        decimal.Decimal                  `json:"savingsTotal"`
	InvestmentTotal     decimal.Decimal                  `json:"investmentTotal"`
	GainTotal           decimal.Decimal                  `json:"gainTotal"`
	TargetSavings       decimal.Decimal                  `json:"targetSavings"`
	MonthlyContribution decimal.Decimal                  `json:"monthlyContribution"`
	ForecastRate        float64                          `json:"forecastRate"`
	SWR                 float64                          `json:"swr"`
	YearlyExpense       decimal.Decimal                  `json:"yearlyExpense"`
	XIRR                decimal.Decimal                  `json:"xirr"`
	Postings            []posting.Posting                `json:"postings"`
	Balances            map[string]assets.AssetBreakdown `json:"balances"`
	Intelligence        service.GoalIntelligence         `json:"intelligence"`
}

// SavingsGoalDetail is the JSON body returned by
// GET /api/goals/savings/:name. TargetDate is a configured date-only string
// (YYYY-MM-DD); the timeline dates are RFC3339 strings on the wire.
type SavingsGoalDetail struct {
	Type                string                           `json:"type"`
	Name                string                           `json:"name"`
	Icon                string                           `json:"icon"`
	InvestmentTotal     decimal.Decimal                  `json:"investmentTotal"`
	SavingsTotal        decimal.Decimal                  `json:"savingsTotal"`
	GainTotal           decimal.Decimal                  `json:"gainTotal"`
	SavingsTimeline     []accounting.Point               `json:"savingsTimeline"`
	Target              decimal.Decimal                  `json:"target"`
	TargetDate          string                           `json:"targetDate"`
	Rate                float64                          `json:"rate"`
	PaymentPerPeriod    float64                          `json:"paymentPerPeriod"`
	XIRR                decimal.Decimal                  `json:"xirr"`
	Postings            []posting.Posting                `json:"postings"`
	Balances            map[string]assets.AssetBreakdown `json:"balances"`
	ContributionHistory []service.GoalContribution       `json:"contributionHistory"`
	Intelligence        service.GoalIntelligence         `json:"intelligence"`
}

// nonNilPoints keeps a nil running-balance series out of the JSON payload; the
// generated TypeScript type declares an array, not null.
func nonNilPoints(points []accounting.Point) []accounting.Point {
	if points == nil {
		return []accounting.Point{}
	}
	return points
}

// nonNilPostings is the posting-list counterpart of nonNilPoints.
func nonNilPostings(postings []posting.Posting) []posting.Posting {
	if postings == nil {
		return []posting.Posting{}
	}
	return postings
}
