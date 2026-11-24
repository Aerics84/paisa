package server

import (
	"github.com/ananthakumaran/paisa/internal/model/posting"
)

// IncomeResponse is the JSON body returned by GET /api/income.
type IncomeResponse struct {
	IncomeTimeline []Income           `json:"income_timeline"`
	TaxTimeline    []Tax              `json:"tax_timeline"`
	YearlyCards    []IncomeYearlyCard `json:"yearly_cards"`
}

// ExpenseBreakdown groups postings by month or financial year for
// GET /api/expense.
type ExpenseBreakdown struct {
	Expenses    map[string][]posting.Posting `json:"expenses"`
	Incomes     map[string][]posting.Posting `json:"incomes"`
	Investments map[string][]posting.Posting `json:"investments"`
	Taxes       map[string][]posting.Posting `json:"taxes"`
}

// ExpenseResponse is the JSON body returned by GET /api/expense.
type ExpenseResponse struct {
	Expenses  []posting.Posting `json:"expenses"`
	MonthWise ExpenseBreakdown  `json:"month_wise"`
	YearWise  ExpenseBreakdown  `json:"year_wise"`
	Graph     map[string]Graph  `json:"graph"`
}

// CashFlowResponse is the JSON body returned by GET /api/cash_flow.
type CashFlowResponse struct {
	CashFlows []CashFlow `json:"cash_flows"`
}

// IncomeStatementResponse is the JSON body returned by GET
// /api/income_statement.
type IncomeStatementResponse struct {
	Yearly map[string]IncomeStatement `json:"yearly"`
}

// RecurringResponse is the JSON body returned by GET /api/recurring.
type RecurringResponse struct {
	TransactionSequences []TransactionSequence `json:"transaction_sequences"`
}

// emptyPostings turns a nil posting slice into an empty one so the API never
// emits JSON null where the generated TypeScript type declares an array.
func emptyPostings(postings []posting.Posting) []posting.Posting {
	if postings == nil {
		return []posting.Posting{}
	}
	return postings
}
