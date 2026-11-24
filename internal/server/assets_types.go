package server

import (
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/ananthakumaran/paisa/internal/server/assets"
	"github.com/shopspring/decimal"
)

// NetworthResponse is the JSON body returned by GET /api/networth. Dates are
// serialized as RFC3339 strings and amounts as JSON numbers.
type NetworthResponse struct {
	NetworthTimeline  []Networth         `json:"networthTimeline"`
	CurrencyNetworths []CurrencyNetworth `json:"currencyNetworths"`
	XIRR              decimal.Decimal    `json:"xirr"`
}

// InvestmentResponse is the JSON body returned by GET /api/investment.
type InvestmentResponse struct {
	Assets      []posting.Posting      `json:"assets"`
	YearlyCards []InvestmentYearlyCard `json:"yearly_cards"`
}

// GainResponse is the JSON body returned by GET /api/gain.
type GainResponse struct {
	GainBreakdown []Gain `json:"gain_breakdown"`
}

// AccountGainResponse is the JSON body returned by GET /api/gain/:account.
type AccountGainResponse struct {
	GainTimelineBreakdown AccountGain               `json:"gain_timeline_breakdown"`
	PortfolioAllocation   PortfolioAllocationGroups `json:"portfolio_allocation"`
	AssetBreakdown        assets.AssetBreakdown     `json:"asset_breakdown"`
}

// AllocationResponse is the JSON body returned by GET /api/allocation.
// Rebalance is omitted when no allocation targets are configured, matching the
// generated optional TypeScript field.
type AllocationResponse struct {
	Aggregates         map[string]Aggregate   `json:"aggregates"`
	AggregatesTimeline []map[string]Aggregate `json:"aggregates_timeline"`
	AllocationTargets  []AllocationTarget     `json:"allocation_targets"`
	Rebalance          *RebalancePlan         `json:"rebalance,omitempty"`
}

// SaveAllocationTargetsResponse is the success body returned by
// POST /api/allocation/targets.
type SaveAllocationTargetsResponse struct {
	Success bool `json:"success"`
}
