package goal

import (
	"strings"
	"time"

	"github.com/ananthakumaran/paisa/internal/accounting"
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/ananthakumaran/paisa/internal/service"
	"github.com/ananthakumaran/paisa/internal/utils"
	"github.com/shopspring/decimal"
)

// contributionHistoryMonths is the number of complete months before the
// current month that contribute to a savings goal's contribution pace.
const contributionHistoryMonths = 6

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

// contributionHistory sums the savings-account postings of each complete month
// before the current month. Months after the first account activity are kept
// even when no contribution was made, so a missing month is observed zero
// activity rather than absent data.
func contributionHistory(ps []posting.Posting, accounts []string, asOf time.Time) []service.GoalContribution {
	end := utils.BeginningOfMonth(asOf)
	start := end.AddDate(0, -contributionHistoryMonths, 0)
	first := end
	amounts := map[string]decimal.Decimal{}

	selected := accounting.FilterByGlob(ps, accounts)
	for _, p := range selected {
		if p.Forecast || !p.Date.Before(end) {
			continue
		}
		if strings.HasPrefix(p.Account, "Assets:") && p.Date.Before(first) {
			first = p.Date
		}
		if p.Date.Before(start) {
			continue
		}
		month := p.Date.Format("2006-01")
		amounts[month] = amounts[month].Add(p.Amount)
	}

	if first.After(start) {
		start = utils.BeginningOfMonth(first)
	}

	result := make([]service.GoalContribution, 0, contributionHistoryMonths)
	for month := start; month.Before(end); month = month.AddDate(0, 1, 0) {
		key := month.Format("2006-01")
		amount, _ := amounts[key].Float64()
		result = append(result, service.GoalContribution{Month: key, Amount: amount})
	}
	return result
}
