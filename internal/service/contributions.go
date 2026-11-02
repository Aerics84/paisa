package service

import (
	"path/filepath"
	"time"

	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/ananthakumaran/paisa/internal/utils"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type ContributionMonth struct {
	Month  string          `json:"month"`
	Amount decimal.Decimal `json:"amount"`
}

// MonthlyContributions measures cash flow, never changes in market value.
// ps must include Assets and Income:CapitalGains postings. Capital gains are
// matched against their source asset account: a sale's proceeds retained inside
// the selected account set then cancel its cost and realized gain exactly.
// Completed months after the first asset activity are retained, including zeros.
func MonthlyContributions(db *gorm.DB, ps []posting.Posting, accounts []string, asOf time.Time) []ContributionMonth {
	return monthlyContributions(ps, accounts, asOf, func(window []posting.Posting) []investmentEvent { return loadInvestmentEvents(db, window) })
}

// monthlyContributions permits a captured snapshot to supply its own counterparts.
func monthlyContributions(ps []posting.Posting, accounts []string, asOf time.Time, classify func([]posting.Posting) []investmentEvent) []ContributionMonth {
	end := utils.BeginningOfMonth(asOf)
	start := end.AddDate(0, -6, 0)
	first := end
	amounts := map[string]decimal.Decimal{}
	window := make([]posting.Posting, 0)
	selected := filterContributions(ps, accounts)
	for i := range selected {
		p := &selected[i]
		if p.Forecast || !p.Date.Before(end) {
			continue
		}
		if utils.IsParent(p.Account, "Assets") && p.Date.Before(first) {
			first = p.Date
		}
		if p.Date.Before(start) {
			continue
		}
		window = append(window, *p)
	}
	events := classify(window)
	for i := range events {
		e := &events[i]
		month := e.Posting.Date.Format("2006-01")
		amounts[month] = amounts[month].Add(e.ActivityFlow)
	}

	if first.After(start) {
		start = utils.BeginningOfMonth(first)
	}
	result := make([]ContributionMonth, 0, 6)
	for month := start; month.Before(end); month = month.AddDate(0, 1, 0) {
		key := month.Format("2006-01")
		result = append(result, ContributionMonth{Month: key, Amount: amounts[key]})
	}
	return result
}

// filterContributions matches capital-gains postings against the asset account
// they belong to. This fork's accounting.FilterByGlob does not remap them, so
// the reference behaviour is kept local to contribution accounting.
func filterContributions(ps []posting.Posting, accounts []string) []posting.Posting {
	globals := make([]string, 0, len(accounts))
	for _, account := range accounts {
		if account != "" {
			globals = append(globals, account)
		}
	}
	if len(globals) == 0 {
		return ps
	}

	negatePresent := false
	for _, glob := range globals {
		if glob[0] == '!' {
			negatePresent = true
			break
		}
	}

	selected := make([]posting.Posting, 0, len(ps))
	for _, p := range ps {
		matched := negatePresent
		for _, glob := range globals {
			negative := false
			if glob[0] == '!' {
				negative = true
				glob = glob[1:]
			}

			account := p.Account
			if utils.IsParent(p.Account, "Income:CapitalGains") {
				account = CapitalGainsSourceAccount(p.Account)
			}
			match, err := filepath.Match(glob, account)
			if err != nil {
				match = false
			}
			if negative {
				match = !match
			}

			if negatePresent {
				if !match {
					matched = false
					break
				}
			} else if match {
				matched = true
				break
			}
		}
		if matched {
			selected = append(selected, p)
		}
	}
	return selected
}
