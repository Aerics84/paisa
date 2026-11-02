package goal

import (
	"time"

	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/ananthakumaran/paisa/internal/service"
	"github.com/ananthakumaran/paisa/internal/utils"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// analyze evaluates a goal as of today. The frontend only displays the result.
func analyze(goalType string, current, target decimal.Decimal, targetDate string, rate, payment float64, history []service.GoalContribution) service.GoalIntelligence {
	currentValue, _ := current.Float64()
	targetValue, _ := target.Float64()

	return service.AnalyzeGoal(service.GoalInput{
		Type:                goalType,
		Current:             currentValue,
		Target:              targetValue,
		TargetDate:          targetDate,
		Rate:                rate,
		PaymentPerPeriod:    payment,
		ContributionHistory: history,
	}, utils.Now())
}

// contributionHistory measures goal cash flow for the last six complete months.
// Market value changes, interest, dividends and internal sales are not
// contributions; see service.MonthlyContributions for the classification.
func contributionHistory(db *gorm.DB, ps []posting.Posting, accounts []string, asOf time.Time) []service.GoalContribution {
	months := service.MonthlyContributions(db, ps, accounts, asOf)
	result := make([]service.GoalContribution, 0, len(months))
	for _, month := range months {
		amount, _ := month.Amount.Float64()
		result = append(result, service.GoalContribution{Month: month.Month, Amount: amount})
	}
	return result
}
