package goal

import (
	"github.com/ananthakumaran/paisa/internal/accounting"
	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/ananthakumaran/paisa/internal/query"
	"github.com/ananthakumaran/paisa/internal/server/assets"
	"github.com/ananthakumaran/paisa/internal/service"
	"github.com/ananthakumaran/paisa/internal/utils"
	"github.com/gin-gonic/gin"
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

func getSavingsDetail(db *gorm.DB, conf config.SavingsGoal) gin.H {
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

	return gin.H{
		"type":                "savings",
		"name":                conf.Name,
		"icon":                conf.Icon,
		"investmentTotal":     investmentTotal,
		"savingsTotal":        savingsTotal,
		"gainTotal":           savingsTotal.Sub(investmentTotal),
		"savingsTimeline":     accounting.RunningBalance(db, savings),
		"target":              decimal.NewFromFloat(conf.Target),
		"targetDate":          conf.TargetDate,
		"rate":                conf.Rate,
		"paymentPerPeriod":    conf.PaymentPerPeriod,
		"xirr":                service.XIRR(db, savingsWithCapitalGains),
		"postings":            savingsWithCapitalGains,
		"balances":            balances,
		"contributionHistory": history,
		"intelligence":        analyze("savings", savingsTotal, decimal.NewFromFloat(conf.Target), conf.TargetDate, conf.Rate, conf.PaymentPerPeriod, history),
	}
}
