package server

import (
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/ananthakumaran/paisa/internal/server/liabilities"
)

// LiabilityInterestResponse is the JSON body returned by
// GET /api/liabilities/interest.
type LiabilityInterestResponse struct {
	InterestTimelineBreakdown []liabilities.Interest `json:"interest_timeline_breakdown"`
}

// LiabilityBalanceResponse is the JSON body returned by
// GET /api/liabilities/balance. The breakdowns are keyed by account group.
type LiabilityBalanceResponse struct {
	LiabilityBreakdowns map[string]liabilities.AssetBreakdown `json:"liability_breakdowns"`
}

// LiabilityRepaymentResponse is the JSON body returned by
// GET /api/liabilities/repayment.
type LiabilityRepaymentResponse struct {
	Repayments []posting.Posting `json:"repayments"`
}

// CreditCardsResponse is the JSON body returned by GET /api/credit_cards.
type CreditCardsResponse struct {
	CreditCards []CreditCardSummary `json:"creditCards"`
}

// CreditCardResponse is the JSON body returned by GET /api/credit_cards/:account.
// CreditCard is omitted when the configured card cannot be found.
type CreditCardResponse struct {
	CreditCard *CreditCardSummary `json:"creditCard,omitempty"`
	Found      bool               `json:"found"`
}
