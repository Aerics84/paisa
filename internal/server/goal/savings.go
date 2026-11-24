package goal

import (
	"github.com/ananthakumaran/paisa/internal/accounting"
	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/ananthakumaran/paisa/internal/query"
	"github.com/ananthakumaran/paisa/internal/server/assets"
	"github.com/ananthakumaran/paisa/internal/service"
	"github.com/ananthakumaran/paisa/internal/utils"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func getSavingsSummary(db *gorm.DB, ps []posting.Posting, conf config.SavingsGoal) GoalSummary {
	savings := accounting.FilterByGlob(ps, conf.Accounts)
	savingsTotal := accounting.CurrentBalance(savings)

	return GoalSummary{
		Type:       "savings",
		Id:         "savings-" + conf.Name,
		Name:       conf.Name,
		Current:    savingsTotal,
		Target:     decimal.NewFromFloat(conf.Target),
		TargetDate: conf.TargetDate,
		Icon:       conf.Icon,
		Priority:   conf.Priority,
	}
}

func getSavingsDetail(db *gorm.DB, conf config.SavingsGoal) SavingsGoalDetail {
	savings := accounting.FilterByGlob(query.Init(db).UntilToday().Like("Assets:%").All(), conf.Accounts)
	savings = service.PopulateMarketPrice(db, savings)
	savingsTotal := accounting.CurrentBalance(savings)
	investmentTotal := accounting.CostBalance(savings)

	// Keep the unfiltered postings for contribution accounting: the capital-gain
	// counterparts must be selected together with their source asset account.
	contributionPostings := query.Init(db).UntilToday().Like("Assets:%", "Income:CapitalGains:%").All()
	savingsWithCapitalGains := accounting.FilterByGlob(contributionPostings, conf.Accounts)
	savingsWithCapitalGains = service.PopulateMarketPrice(db, savingsWithCapitalGains)

	balances := assets.ComputeBreakdowns(db, savingsWithCapitalGains, false)
	history := contributionHistory(db, contributionPostings, conf.Accounts, utils.Now())

	return SavingsGoalDetail{
		Type:                "savings",
		Name:                conf.Name,
		Icon:                conf.Icon,
		InvestmentTotal:     investmentTotal,
		SavingsTotal:        savingsTotal,
		GainTotal:           savingsTotal.Sub(investmentTotal),
		SavingsTimeline:     nonNilPoints(accounting.RunningBalance(db, savings)),
		Target:              decimal.NewFromFloat(conf.Target),
		TargetDate:          conf.TargetDate,
		Rate:                conf.Rate,
		PaymentPerPeriod:    conf.PaymentPerPeriod,
		XIRR:                service.XIRR(db, savingsWithCapitalGains),
		Postings:            nonNilPostings(savingsWithCapitalGains),
		Balances:            balances,
		ContributionHistory: history,
		Intelligence:        analyze("savings", savingsTotal, decimal.NewFromFloat(conf.Target), conf.TargetDate, conf.Rate, conf.PaymentPerPeriod, history),
	}
}
