package server

import (
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/ananthakumaran/paisa/internal/query"
	"github.com/ananthakumaran/paisa/internal/server/assets"
	"github.com/ananthakumaran/paisa/internal/server/goal"
	"github.com/ananthakumaran/paisa/internal/service"
	"github.com/ananthakumaran/paisa/internal/utils"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// ExpensePaceSummary projects the current month's spending at the pace so far.
type ExpensePaceSummary struct {
	Period            string          `json:"period"`
	ProjectedExpenses decimal.Decimal `json:"projectedExpenses"`
	// OverBudget is omitted when the projection is within the budget.
	OverBudget *decimal.Decimal `json:"overBudget,omitempty"`
}

// expensePaceSummary compares this month's spending with the month's budget.
func expensePaceSummary(expenses map[string][]posting.Posting, budget BudgetResponse) *ExpensePaceSummary {
	asOf := utils.Now()
	period := asOf.Format("2006-01")
	spent := utils.SumBy(expenses[period], func(p posting.Posting) decimal.Decimal { return p.Amount })

	var planned *decimal.Decimal
	if current, ok := budget.BudgetsByMonth[period]; ok && len(current.Accounts) > 0 {
		total := utils.SumBy(current.Accounts, func(a AccountBudget) decimal.Decimal { return a.Forecast })
		planned = &total
	}

	pace := service.ProjectExpensePace(spent, planned, asOf)
	if pace == nil {
		return nil
	}
	return &ExpensePaceSummary{
		Period:            pace.Period,
		ProjectedExpenses: pace.ProjectedExpenses,
		OverBudget:        pace.OverBudget,
	}
}

func GetDashboard(db *gorm.DB) DashboardResponse {
	expenses := GetCurrentExpense(db)
	if expenses == nil {
		expenses = map[string][]posting.Posting{}
	}
	budget := GetCurrentBudget(db)
	return DashboardResponse{
		CheckingBalances:        assets.GetCheckingBalance(db),
		CheckingBalanceTimeline: nonNilSlice(assets.GetCheckingBalanceTimeline(db)),
		AvailableMonthRange:     assets.GetAvailableMonthRange(db),
		Networth:                GetCurrentNetworth(db),
		Expenses:                expenses,
		ExpensePace:             expensePaceSummary(expenses, budget),
		CashFlows:               nonNilSlice(GetCurrentCashFlow(db)),
		TransactionSequences:    nonNilSlice(ComputeRecurringTransactions(query.Init(db).All())),
		Transactions:            nonNilSlice(GetLatestTransactions(db)),
		Budget:                  budget,
		GoalSummaries:           nonNilSlice(goal.GetGoalSummaries(db)),
	}
}

// nonNilSlice keeps the JSON shape in sync with the generated TypeScript
// types: a nil Go slice would serialize as null, but the wire contract uses an
// array.
func nonNilSlice[T any](items []T) []T {
	if items == nil {
		return []T{}
	}
	return items
}
