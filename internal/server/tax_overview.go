package server

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/ananthakumaran/paisa/internal/taxcategory"
	"github.com/ananthakumaran/paisa/internal/utils"
	"github.com/shopspring/decimal"
)

type TaxOverview struct {
	Years         []string                 `json:"years"`
	SelectedYear  string                   `json:"selected_year"`
	Summary       TaxOverviewSummary       `json:"summary"`
	Sections      []TaxOverviewSection     `json:"sections"`
	Coverage      TaxOverviewCoverage      `json:"coverage"`
	CapitalIncome TaxOverviewCapitalIncome `json:"capital_income"`
	Diagnostics   []TaxOverviewDiagnostic  `json:"diagnostics"`
}

type TaxOverviewSummary struct {
	ExpenseCandidates       decimal.Decimal `json:"expense_candidates"`
	MatchedExpensePostings  int             `json:"matched_expense_posting_count"`
	CapitalIncomeBooked     decimal.Decimal `json:"capital_income_booked"`
	CapitalIncomeCalculated decimal.Decimal `json:"capital_income_calculated"`
	DiagnosticCount         int             `json:"diagnostic_count"`
}

type TaxOverviewSection struct {
	Key          string                `json:"key"`
	Label        string                `json:"label"`
	Form         string                `json:"form"`
	Total        decimal.Decimal       `json:"total"`
	PostingCount int                   `json:"posting_count"`
	Categories   []TaxOverviewCategory `json:"categories"`
}

type TaxOverviewCategory struct {
	Key      string               `json:"key"`
	Label    string               `json:"label"`
	Total    decimal.Decimal      `json:"total"`
	Postings []TaxOverviewPosting `json:"postings"`
}

type TaxOverviewPosting struct {
	TransactionID        string          `json:"transaction_id"`
	Date                 time.Time       `json:"date"`
	Payee                string          `json:"payee"`
	Account              string          `json:"account"`
	Amount               decimal.Decimal `json:"amount"`
	Note                 string          `json:"note"`
	FileName             string          `json:"file_name"`
	TransactionBeginLine uint64          `json:"transaction_begin_line"`
	Source               string          `json:"source"`
	Category             string          `json:"category"`
}

type TaxOverviewCoverage struct {
	AccountsWithUnmatchedPostings int                          `json:"accounts_with_unmatched_postings"`
	UnmatchedPostingCount         int                          `json:"unmatched_posting_count"`
	Accounts                      []TaxOverviewCoverageAccount `json:"accounts"`
}

type TaxOverviewCoverageAccount struct {
	Account               string `json:"account"`
	UnmatchedPostingCount int    `json:"unmatched_posting_count"`
}

type TaxOverviewCapitalIncome struct {
	RealizedGains  decimal.Decimal                   `json:"realized_gains"`
	RealizedLosses decimal.Decimal                   `json:"realized_losses"`
	Dividends      decimal.Decimal                   `json:"dividends"`
	Interest       decimal.Decimal                   `json:"interest"`
	TaxesBooked    decimal.Decimal                   `json:"taxes_booked"`
	Sales          int                               `json:"sales"`
	Accounts       []TaxOverviewCapitalIncomeAccount `json:"accounts"`
}

type TaxOverviewCapitalIncomeAccount struct {
	Account      string          `json:"account"`
	RealizedGain decimal.Decimal `json:"realized_gain"`
}

type TaxOverviewDiagnostic struct {
	Level   string `json:"level"`
	Summary string `json:"summary"`
	Details string `json:"details"`
}

// Internal resolution states. These never appear in the API.
type taxResolutionStatus string

const (
	taxMatched    taxResolutionStatus = "matched"
	taxExcluded   taxResolutionStatus = "excluded"
	taxUnresolved taxResolutionStatus = "unresolved"
	taxInvalid    taxResolutionStatus = "invalid"
)

type taxResolution struct {
	Status     taxResolutionStatus
	Category   string
	Source     string
	Diagnostic *TaxOverviewDiagnostic
}

const negativePostingDiagnostic = "Negative Aufwandsposten erkannt. Diese können Erstattungen oder Korrekturen sein. Die steuerliche Behandlung von Erstattungen in einem anderen Jahr hängt von der Art der Ausgabe und den Umständen ab. Bitte manuell prüfen; Paisa bestimmt nicht das richtige Steuerjahr."

func unknownCategoryDiagnostic(value string, count int, examples string) TaxOverviewDiagnostic {
	return TaxOverviewDiagnostic{
		Level:   "warning",
		Summary: fmt.Sprintf("Unbekannte Steuerkategorie `%s` bei %d Postings; nicht angezeigt.", value, count),
		Details: examples,
	}
}

func ambiguousTagDiagnostic(count int, examples string) TaxOverviewDiagnostic {
	return TaxOverviewDiagnostic{
		Level:   "warning",
		Summary: fmt.Sprintf("%d Transaktionen haben ein Steuer-Tag, aber nicht genau einen Aufwandsposten; Transaktion aufteilen oder den Aufwandsposten taggen.", count),
		Details: examples,
	}
}

func resolveTaxPosting(p posting.Posting, siblings []posting.Posting, rules []config.TaxDeductionAccount) taxResolution {
	if p.TaxCategory == "none" {
		return taxResolution{Status: taxExcluded}
	}

	if p.TaxCategory != "" {
		if taxcategory.IsKnownCategory(p.TaxCategory) {
			return taxResolution{Status: taxMatched, Category: p.TaxCategory, Source: "tag"}
		}
		diagnostic := unknownCategoryDiagnostic(p.TaxCategory, 1, taxPostingExamples([]posting.Posting{p}, 5))
		return taxResolution{Status: taxInvalid, Category: p.TaxCategory, Diagnostic: &diagnostic}
	}

	expensePostings := 0
	siblingValues := []string{}
	seenValues := map[string]bool{}
	for _, sibling := range siblings {
		if utils.IsSameOrParent(sibling.Account, "Expenses") {
			expensePostings++
			continue
		}
		if sibling.TaxCategory == "" || seenValues[sibling.TaxCategory] {
			continue
		}
		seenValues[sibling.TaxCategory] = true
		siblingValues = append(siblingValues, sibling.TaxCategory)
	}

	if len(siblingValues) == 1 {
		value := siblingValues[0]
		if expensePostings > 1 {
			diagnostic := ambiguousTagDiagnostic(1, taxPostingExamples([]posting.Posting{p}, 5))
			return taxResolution{Status: taxInvalid, Diagnostic: &diagnostic}
		}
		switch {
		case value == "none":
			return taxResolution{Status: taxExcluded}
		case taxcategory.IsKnownCategory(value):
			return taxResolution{Status: taxMatched, Category: value, Source: "transaction"}
		default:
			diagnostic := unknownCategoryDiagnostic(value, 1, taxPostingExamples([]posting.Posting{p}, 5))
			return taxResolution{Status: taxInvalid, Category: value, Diagnostic: &diagnostic}
		}
	}

	if len(siblingValues) > 1 {
		diagnostic := ambiguousTagDiagnostic(1, taxPostingExamples([]posting.Posting{p}, 5))
		return taxResolution{Status: taxInvalid, Diagnostic: &diagnostic}
	}

	bestLength := -1
	bestRule := config.TaxDeductionAccount{}
	for _, rule := range rules {
		if p.Account == rule.Account || strings.HasPrefix(p.Account, rule.Account+":") {
			if len(rule.Account) > bestLength {
				bestLength = len(rule.Account)
				bestRule = rule
			}
		}
	}
	if bestLength >= 0 {
		return taxResolution{Status: taxMatched, Category: bestRule.Category, Source: "account"}
	}

	return taxResolution{Status: taxUnresolved}
}

func TaxOverviewYears(postings []posting.Posting, now time.Time) []string {
	years := map[string]bool{now.AddDate(-1, 0, 0).Format("2006"): true}
	for _, p := range postings {
		if p.Date.IsZero() {
			continue
		}
		if p.Date.Year() < 1900 || p.Date.Year() > 2100 {
			continue
		}
		years[p.Date.Format("2006")] = true
	}
	return utils.SortedKeys(years)
}

func BuildTaxOverview(postings []posting.Posting, year string, rules []config.TaxDeductionAccount) TaxOverview {
	overview := TaxOverview{
		SelectedYear:  year,
		Sections:      []TaxOverviewSection{},
		Coverage:      TaxOverviewCoverage{Accounts: []TaxOverviewCoverageAccount{}},
		Diagnostics:   []TaxOverviewDiagnostic{},
		Summary:       TaxOverviewSummary{},
		CapitalIncome: TaxOverviewCapitalIncome{Accounts: []TaxOverviewCapitalIncomeAccount{}},
	}

	sectionIndex := map[string]int{}
	categoryIndex := map[string]map[string]int{}
	for _, section := range taxcategory.Sections {
		if section.Key == taxcategory.CapitalIncomeSection {
			continue
		}

		categories := []TaxOverviewCategory{}
		index := map[string]int{}
		for _, category := range taxcategory.CategoriesForSection(section.Key) {
			index[category.Key] = len(categories)
			categories = append(categories, TaxOverviewCategory{
				Key:      category.Key,
				Label:    category.Label,
				Total:    decimal.Zero,
				Postings: []TaxOverviewPosting{},
			})
		}

		sectionIndex[section.Key] = len(overview.Sections)
		categoryIndex[section.Key] = index
		overview.Sections = append(overview.Sections, TaxOverviewSection{
			Key:          section.Key,
			Label:        section.Label,
			Form:         section.Form,
			Total:        decimal.Zero,
			PostingCount: 0,
			Categories:   categories,
		})
	}

	byTransaction := map[string][]posting.Posting{}
	for _, p := range postings {
		byTransaction[p.TransactionID] = append(byTransaction[p.TransactionID], p)
	}

	selected := []posting.Posting{}
	for _, p := range postings {
		if p.Date.Format("2006") == year {
			selected = append(selected, p)
		}
	}

	unknown := map[string][]posting.Posting{}
	unknownOrder := []string{}
	ambiguous := []posting.Posting{}
	legacy := []posting.Posting{}
	negative := []posting.Posting{}
	zero := []posting.Posting{}
	unresolvedByAccount := map[string]int{}

	for _, p := range selected {
		if p.TaxCategoryLegacy != "" {
			legacy = append(legacy, p)
		}
		if !utils.IsSameOrParent(p.Account, "Expenses") {
			continue
		}

		resolution := resolveTaxPosting(p, byTransaction[p.TransactionID], rules)
		switch resolution.Status {
		case taxMatched:
			category, ok := taxcategory.CategoryByKey(resolution.Category)
			if !ok {
				continue
			}
			sectionIdx, ok := sectionIndex[category.Section]
			if !ok {
				continue
			}
			categoryIdx, ok := categoryIndex[category.Section][category.Key]
			if !ok {
				continue
			}

			overview.Sections[sectionIdx].Categories[categoryIdx].Postings = append(
				overview.Sections[sectionIdx].Categories[categoryIdx].Postings,
				TaxOverviewPosting{
					TransactionID:        p.TransactionID,
					Date:                 p.Date,
					Payee:                p.Payee,
					Account:              p.Account,
					Amount:               p.Amount,
					Note:                 p.Note,
					FileName:             p.FileName,
					TransactionBeginLine: p.TransactionBeginLine,
					Source:               resolution.Source,
					Category:             resolution.Category,
				},
			)
			overview.Sections[sectionIdx].Categories[categoryIdx].Total = overview.Sections[sectionIdx].Categories[categoryIdx].Total.Add(p.Amount)
			overview.Sections[sectionIdx].Total = overview.Sections[sectionIdx].Total.Add(p.Amount)
			overview.Sections[sectionIdx].PostingCount++
			overview.Summary.ExpenseCandidates = overview.Summary.ExpenseCandidates.Add(p.Amount)
			overview.Summary.MatchedExpensePostings++
			if p.Amount.IsNegative() {
				negative = append(negative, p)
			}
			if p.Amount.IsZero() {
				zero = append(zero, p)
			}
		case taxUnresolved:
			unresolvedByAccount[p.Account]++
		case taxInvalid:
			if resolution.Category != "" && !taxcategory.IsKnownCategory(resolution.Category) {
				if _, ok := unknown[resolution.Category]; !ok {
					unknownOrder = append(unknownOrder, resolution.Category)
				}
				unknown[resolution.Category] = append(unknown[resolution.Category], p)
			} else {
				ambiguous = append(ambiguous, p)
			}
		}
	}

	for _, value := range unknownOrder {
		overview.Diagnostics = append(overview.Diagnostics,
			unknownCategoryDiagnostic(value, len(unknown[value]), taxPostingExamples(unknown[value], 5)))
	}

	if len(ambiguous) > 0 {
		overview.Diagnostics = append(overview.Diagnostics,
			ambiguousTagDiagnostic(distinctTransactionCount(ambiguous), taxPostingExamples(ambiguous, 5)))
	}

	if examples := taggedTransactionsWithoutExpense(selected, byTransaction); len(examples) > 0 {
		overview.Diagnostics = append(overview.Diagnostics,
			ambiguousTagDiagnostic(len(examples), taxPostingExamples(examples, 5)))
	}

	if len(legacy) > 0 {
		overview.Diagnostics = append(overview.Diagnostics, TaxOverviewDiagnostic{
			Level:   "warning",
			Summary: fmt.Sprintf("%d Postings verwenden noch das Legacy-Tag `TaxRelevant`; bitte auf `tax_category` umstellen.", len(legacy)),
			Details: taxPostingExamples(legacy, 5),
		})
	}

	if len(negative) > 0 {
		overview.Diagnostics = append(overview.Diagnostics, TaxOverviewDiagnostic{
			Level:   "warning",
			Summary: negativePostingDiagnostic,
			Details: taxPostingExamples(negative, 5),
		})
	}

	if len(zero) > 0 {
		overview.Diagnostics = append(overview.Diagnostics, TaxOverviewDiagnostic{
			Level:   "warning",
			Summary: fmt.Sprintf("%d erkannte Postings haben einen Betrag von 0.", len(zero)),
			Details: taxPostingExamples(zero, 5),
		})
	}

	coverageAccounts := make([]TaxOverviewCoverageAccount, 0, len(unresolvedByAccount))
	for account, count := range unresolvedByAccount {
		coverageAccounts = append(coverageAccounts, TaxOverviewCoverageAccount{
			Account:               account,
			UnmatchedPostingCount: count,
		})
	}
	sort.Slice(coverageAccounts, func(i, j int) bool {
		if coverageAccounts[i].UnmatchedPostingCount != coverageAccounts[j].UnmatchedPostingCount {
			return coverageAccounts[i].UnmatchedPostingCount > coverageAccounts[j].UnmatchedPostingCount
		}
		return coverageAccounts[i].Account < coverageAccounts[j].Account
	})
	overview.Coverage.Accounts = coverageAccounts
	overview.Coverage.AccountsWithUnmatchedPostings = len(coverageAccounts)
	for _, account := range coverageAccounts {
		overview.Coverage.UnmatchedPostingCount += account.UnmatchedPostingCount
	}

	overview.Summary.DiagnosticCount = len(overview.Diagnostics)
	return overview
}

func distinctTransactionCount(ps []posting.Posting) int {
	seen := map[string]bool{}
	count := 0
	for _, p := range ps {
		if seen[p.TransactionID] {
			continue
		}
		seen[p.TransactionID] = true
		count++
	}
	return count
}

func taggedTransactionsWithoutExpense(postings []posting.Posting, byTransaction map[string][]posting.Posting) []posting.Posting {
	examples := []posting.Posting{}
	seen := map[string]bool{}
	for _, p := range postings {
		if seen[p.TransactionID] || p.TaxCategory == "" {
			continue
		}
		expensePostings := 0
		for _, sibling := range byTransaction[p.TransactionID] {
			if utils.IsSameOrParent(sibling.Account, "Expenses") {
				expensePostings++
			}
		}
		if expensePostings == 0 {
			seen[p.TransactionID] = true
			examples = append(examples, p)
		}
	}
	return examples
}

func taxPostingExamples(ps []posting.Posting, limit int) string {
	examples := []string{}
	for i, p := range ps {
		if i >= limit {
			break
		}
		examples = append(examples, fmt.Sprintf("%s %s (%s)", p.Date.Format("2006-01-02"), p.Payee, p.Account))
	}
	joined := strings.Join(examples, "; ")
	if len(ps) > limit {
		joined += "; …"
	}
	return joined
}
