package goal

import (
	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/ananthakumaran/paisa/internal/query"
	"github.com/ananthakumaran/paisa/internal/service"
	"github.com/ananthakumaran/paisa/internal/utils"
	"github.com/samber/lo"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type GoalSummary struct {
	Type         string                    `json:"type"`
	Name         string                    `json:"name"`
	Id           string                    `json:"id"`
	Icon         string                    `json:"icon"`
	Current      decimal.Decimal           `json:"current"`
	Target       decimal.Decimal           `json:"target"`
	TargetDate   string                    `json:"targetDate"`
	Priority     int                       `json:"priority"`
	Intelligence *service.GoalIntelligence `json:"intelligence,omitempty"`
}

func GetGoalSummaries(db *gorm.DB, store *config.Store) []GoalSummary {
	summaries := []GoalSummary{}
	goals := store.Get().Goals
	assetPostings := query.Init(db).UntilToday().Like("Assets:%").All()
	assetPostings = service.PopulateMarketPrice(db, assetPostings)

	// Keep older asset activity so observed zero months are retained. Only the
	// capital-gain counterparts need a bounded query; they make an internal sale
	// cancel its cost and realized gain exactly.
	contributionPostings := append([]posting.Posting{}, assetPostings...)
	if len(goals.Savings) > 0 {
		end := utils.BeginningOfMonth(utils.Now())
		start := end.AddDate(0, -6, 0)
		gains := query.Init(db).Like("Income:CapitalGains:%").Where("date between ? AND ?", start, end).All()
		contributionPostings = append(contributionPostings, gains...)
	}

	for _, goal := range goals.Retirement {
		summary := getRetirementSummary(db, assetPostings, goal)
		intelligence := analyze(summary.Type, summary.Current, summary.Target, "", 0, 0, nil)
		summary.Intelligence = &intelligence
		summaries = append(summaries, summary)
	}

	for _, goal := range goals.Savings {
		summary := getSavingsSummary(db, assetPostings, goal)
		history := contributionHistory(db, contributionPostings, goal.Accounts, utils.Now())
		intelligence := analyze(summary.Type, summary.Current, summary.Target, goal.TargetDate, goal.Rate, goal.PaymentPerPeriod, history)
		summary.Intelligence = &intelligence
		summaries = append(summaries, summary)
	}

	return summaries
}

// GetGoalDetails returns the detail payload for a configured goal. The return
// type is intentionally dynamic because retirement and savings have different
// response shapes; an unknown type keeps the historical empty-object response.
//
// TODO(api): replace this with an explicit discriminated result if a caller
// ever needs exhaustive goal-kind handling.
func GetGoalDetails(db *gorm.DB, goalType string, name string, store *config.Store) any {
	switch goalType {
	case "retirement":
		conf, _ := lo.Find(store.Get().Goals.Retirement, func(conf config.RetirementGoal) bool { return conf.Name == name })
		return getRetirementDetail(db, conf)
	case "savings":
		conf, _ := lo.Find(store.Get().Goals.Savings, func(conf config.SavingsGoal) bool { return conf.Name == name })
		return getSavingsDetail(db, conf)
	}
	return struct{}{}
}
