package server

import (
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/ananthakumaran/paisa/internal/query"
	"github.com/ananthakumaran/paisa/internal/server/assets"
	"github.com/ananthakumaran/paisa/internal/server/goal"
	"github.com/ananthakumaran/paisa/internal/service"
	"github.com/ananthakumaran/paisa/internal/utils"
	"github.com/gin-gonic/gin"
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
func expensePaceSummary(expenses map[string][]posting.Posting, budget gin.H) *ExpensePaceSummary {
	asOf := utils.Now()
	period := asOf.Format("2006-01")
	spent := utils.SumBy(expenses[period], func(p posting.Posting) decimal.Decimal { return p.Amount })

	var planned *decimal.Decimal
	if budgetsByMonth, ok := budget["budgetsByMonth"].(map[string]Budget); ok {
		if current, ok := budgetsByMonth[period]; ok && len(current.Accounts) > 0 {
			total := utils.SumBy(current.Accounts, func(a AccountBudget) decimal.Decimal { return a.Forecast })
			planned = &total
		}
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

func GetDashboard(db *gorm.DB) gin.H {
	expenses := GetCurrentExpense(db)
	budget := GetCurrentBudget(db)
	return gin.H{
		"checkingBalances":        assets.GetCheckingBalance(db),
		"checkingBalanceTimeline": assets.GetCheckingBalanceTimeline(db),
		"availableMonthRange":     assets.GetAvailableMonthRange(db),
		"networth":                GetCurrentNetworth(db),
		"expenses":                expenses,
		"expense_pace":            expensePaceSummary(expenses, budget),
		"cashFlows":               GetCurrentCashFlow(db),
		"transactionSequences":    ComputeRecurringTransactions(query.Init(db).All()),
		"transactions":            GetLatestTransactions(db),
		"budget":                  budget,
		"goalSummaries":           goal.GetGoalSummaries(db),
	}
}
