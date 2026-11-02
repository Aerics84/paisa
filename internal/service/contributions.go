package service

import (
	"path/filepath"
	"strings"
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

// filterContributions mirrors accounting.FilterByGlob, including its
// capital-gains remapping and its treatment of a plain account as matching
// itself and all subaccounts. It is local only to avoid importing accounting,
// which already depends on this package.
func filterContributions(ps []posting.Posting, accounts []string) []posting.Posting {
	positives := make([]string, 0, len(accounts))
	negatives := make([]string, 0, len(accounts))
	for _, accountGlob := range accounts {
		if accountGlob == "" {
			continue
		}
		if accountGlob[0] == '!' {
			if len(accountGlob) > 1 {
				negatives = append(negatives, accountGlob[1:])
			}
			continue
		}
		positives = append(positives, accountGlob)
	}

	matches := func(globs []string, account string) bool {
		for _, glob := range globs {
			match, err := matchContributionGlob(glob, account)
			if err != nil {
				continue
			}
			if match {
				return true
			}
		}
		return false
	}

	selected := make([]posting.Posting, 0, len(ps))
	for _, p := range ps {
		account := p.Account
		if IsCapitalGains(p) {
			account = CapitalGainsSourceAccount(p.Account)
		}

		if matches(negatives, account) {
			continue
		}
		if len(positives) == 0 || matches(positives, account) {
			selected = append(selected, p)
		}
	}
	return selected
}

// matchContributionGlob keeps accounting.FilterByGlob's semantics: a pattern
// containing "*" is a filepath glob, while a plain account also matches all of
// its subaccounts.
func matchContributionGlob(pattern string, account string) (bool, error) {
	if strings.Contains(pattern, "*") {
		return filepath.Match(pattern, account)
	}
	return utils.IsSameOrParent(account, pattern), nil
}
