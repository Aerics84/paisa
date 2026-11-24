package server

import (
	"time"

	"github.com/ananthakumaran/paisa/internal/accounting"
	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/ananthakumaran/paisa/internal/model/transaction"
	"github.com/ananthakumaran/paisa/internal/server/assets"
	"github.com/ananthakumaran/paisa/internal/server/goal"
	"github.com/shopspring/decimal"
)

// ConfigResponse is the JSON body returned by GET /api/config.
type ConfigResponse struct {
	Config   config.Config `json:"config"`
	Accounts []string      `json:"accounts"`
	Now      *time.Time    `json:"now,omitempty"`
	Schema   any           `json:"schema"`
}

// LedgerResponse is the JSON body returned by GET /api/ledger. Dates are
// serialized as RFC3339 strings; see internal/server/json.go.
type LedgerResponse struct {
	Postings []posting.Posting `json:"postings"`
}

// TransactionResponse is the JSON body returned by GET /api/transaction.
type TransactionResponse struct {
	Transactions []transaction.Transaction `json:"transactions"`
}

// BalanceResponse is the JSON body returned by GET /api/transaction/balanced.
type BalanceResponse struct {
	BalancedPostings []accounting.BalancedPosting `json:"balancedPostings"`
}

// CurrentNetworth is the live snapshot nested in DashboardResponse.
type CurrentNetworth struct {
	Networth Networth        `json:"networth"`
	XIRR     decimal.Decimal `json:"xirr"`
}

// BudgetResponse is the JSON body returned by GET /api/budget and nested in
// DashboardResponse.
type BudgetResponse struct {
	BudgetsByMonth        map[string]Budget `json:"budgetsByMonth"`
	CheckingBalance       decimal.Decimal   `json:"checkingBalance"`
	AvailableForBudgeting decimal.Decimal   `json:"availableForBudgeting"`
}

// DashboardResponse is the JSON body returned by GET /api/dashboard.
type DashboardResponse struct {
	CheckingBalances        assets.BalanceSummary         `json:"checkingBalances"`
	CheckingBalanceTimeline []assets.CheckingBalancePoint `json:"checkingBalanceTimeline"`
	AvailableMonthRange     assets.AvailableMonthRange    `json:"availableMonthRange"`
	Networth                CurrentNetworth               `json:"networth"`
	Expenses                map[string][]posting.Posting  `json:"expenses"`
	// ExpensePace is nil when the current month has no meaningful projection.
	ExpensePace          *ExpensePaceSummary       `json:"expense_pace"`
	CashFlows            []CashFlow                `json:"cashFlows"`
	TransactionSequences []TransactionSequence     `json:"transactionSequences"`
	Transactions         []transaction.Transaction `json:"transactions"`
	Budget               BudgetResponse            `json:"budget"`
	GoalSummaries        []goal.GoalSummary        `json:"goalSummaries"`
}
