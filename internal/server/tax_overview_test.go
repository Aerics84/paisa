package server

import (
	"strings"
	"testing"
	"time"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func taxOverviewPosting(txn, date, account, amount, category, legacy string) posting.Posting {
	d, _ := time.Parse("2006-01-02", date)
	return posting.Posting{
		TransactionID:     txn,
		Date:              d,
		Payee:             txn,
		Account:           account,
		Amount:            decimal.RequireFromString(amount),
		TaxCategory:       category,
		TaxCategoryLegacy: legacy,
	}
}

func taxRules(pairs ...string) []config.TaxDeductionAccount {
	rules := []config.TaxDeductionAccount{}
	for i := 0; i+1 < len(pairs); i += 2 {
		rules = append(rules, config.TaxDeductionAccount{Account: pairs[i], Category: pairs[i+1]})
	}
	return rules
}

func findCategory(t *testing.T, overview TaxOverview, section, category string) TaxOverviewCategory {
	t.Helper()
	for _, s := range overview.Sections {
		if s.Key != section {
			continue
		}
		for _, c := range s.Categories {
			if c.Key == category {
				return c
			}
		}
	}
	t.Fatalf("category %s/%s not found", section, category)
	return TaxOverviewCategory{}
}

func findSection(t *testing.T, overview TaxOverview, key string) TaxOverviewSection {
	t.Helper()
	for _, s := range overview.Sections {
		if s.Key == key {
			return s
		}
	}
	t.Fatalf("section %s not found", key)
	return TaxOverviewSection{}
}

func diagnosticSummaries(overview TaxOverview) string {
	parts := []string{}
	for _, d := range overview.Diagnostics {
		parts = append(parts, d.Summary)
	}
	return strings.Join(parts, " | ")
}

func TestBuildTaxOverviewTagWinsOverAccountRule(t *testing.T) {
	rules := taxRules("Expenses:Health", "health")
	postings := []posting.Posting{
		taxOverviewPosting("t1", "2025-05-01", "Expenses:Health", "100", "work_equipment", ""),
	}
	overview := BuildTaxOverview(postings, "2025", rules)
	c := findCategory(t, overview, "work_expenses", "work_equipment")
	require.Len(t, c.Postings, 1)
	assert.Equal(t, "tag", c.Postings[0].Source)
	assert.True(t, c.Total.Equal(decimal.RequireFromString("100")))
	assert.True(t, findCategory(t, overview, "extraordinary", "health").Total.IsZero())
}

func TestBuildTaxOverviewNoneExcludesWithoutCoverage(t *testing.T) {
	rules := taxRules("Expenses:Health", "health")
	overview := BuildTaxOverview([]posting.Posting{
		taxOverviewPosting("t1", "2025-05-01", "Expenses:Health", "100", "none", ""),
	}, "2025", rules)
	assert.Equal(t, 0, overview.Coverage.UnmatchedPostingCount)
	assert.Empty(t, overview.Diagnostics)
	assert.True(t, overview.Summary.ExpenseCandidates.IsZero())
}

func TestBuildTaxOverviewUnknownCategoryIsDiagnosed(t *testing.T) {
	overview := BuildTaxOverview([]posting.Posting{
		taxOverviewPosting("t1", "2025-05-01", "Expenses:Health", "100", "made_up", ""),
	}, "2025", nil)
	assert.Contains(t, diagnosticSummaries(overview), "Unbekannte Steuerkategorie")
	assert.Equal(t, 0, overview.Coverage.UnmatchedPostingCount)
	assert.True(t, overview.Summary.ExpenseCandidates.IsZero())
}

func TestBuildTaxOverviewSiblingTagOnSingleExpense(t *testing.T) {
	postings := []posting.Posting{
		taxOverviewPosting("t1", "2025-05-01", "Assets:Checking", "-100", "health", ""),
		taxOverviewPosting("t1", "2025-05-01", "Expenses:Health", "100", "", ""),
	}
	overview := BuildTaxOverview(postings, "2025", nil)
	c := findCategory(t, overview, "extraordinary", "health")
	require.Len(t, c.Postings, 1)
	assert.Equal(t, "transaction", c.Postings[0].Source)
	assert.Empty(t, overview.Diagnostics)
}

func TestBuildTaxOverviewSiblingTagIgnoresEmptySiblings(t *testing.T) {
	postings := []posting.Posting{
		taxOverviewPosting("t1", "2025-05-01", "Assets:Checking", "-100", "health", ""),
		taxOverviewPosting("t1", "2025-05-01", "Assets:Checking:Other", "0", "", ""),
		taxOverviewPosting("t1", "2025-05-01", "Expenses:Health", "100", "", ""),
	}
	overview := BuildTaxOverview(postings, "2025", nil)
	require.Len(t, findCategory(t, overview, "extraordinary", "health").Postings, 1)
	assert.NotContains(t, diagnosticSummaries(overview), "nicht genau einen Aufwandsposten")
}

func TestBuildTaxOverviewSiblingNoneExcludes(t *testing.T) {
	postings := []posting.Posting{
		taxOverviewPosting("t1", "2025-05-01", "Assets:Checking", "-100", "none", ""),
		taxOverviewPosting("t1", "2025-05-01", "Expenses:Health", "100", "", ""),
	}
	overview := BuildTaxOverview(postings, "2025", nil)
	assert.Equal(t, 0, overview.Coverage.UnmatchedPostingCount)
	assert.Empty(t, overview.Diagnostics)
	assert.True(t, overview.Summary.ExpenseCandidates.IsZero())
}

func TestBuildTaxOverviewSiblingTagWithTwoExpensesIsAmbiguous(t *testing.T) {
	postings := []posting.Posting{
		taxOverviewPosting("t1", "2025-05-01", "Assets:Checking", "-100", "health", ""),
		taxOverviewPosting("t1", "2025-05-01", "Expenses:Health", "60", "", ""),
		taxOverviewPosting("t1", "2025-05-01", "Expenses:Health:Medicine", "40", "", ""),
	}
	overview := BuildTaxOverview(postings, "2025", nil)
	assert.Contains(t, diagnosticSummaries(overview), "nicht genau einen Aufwandsposten")
	assert.Equal(t, 1, len(overview.Diagnostics))
	assert.Equal(t, 0, overview.Coverage.UnmatchedPostingCount)
	assert.True(t, overview.Summary.ExpenseCandidates.IsZero())
}

func TestBuildTaxOverviewAccountPrefixBoundary(t *testing.T) {
	rules := taxRules("Expenses:Health", "health")
	overview := BuildTaxOverview([]posting.Posting{
		taxOverviewPosting("t1", "2025-05-01", "Expenses:HealthSomething", "100", "", ""),
	}, "2025", rules)
	assert.True(t, overview.Summary.ExpenseCandidates.IsZero())
	assert.Equal(t, 1, overview.Coverage.UnmatchedPostingCount)
	require.Len(t, overview.Coverage.Accounts, 1)
	assert.Equal(t, "Expenses:HealthSomething", overview.Coverage.Accounts[0].Account)
}

func TestBuildTaxOverviewLongestPrefixWins(t *testing.T) {
	rules := taxRules(
		"Expenses:Health", "health",
		"Expenses:Health:Insurance", "health_insurance",
	)
	overview := BuildTaxOverview([]posting.Posting{
		taxOverviewPosting("t1", "2025-05-01", "Expenses:Health:Insurance:Company", "100", "", ""),
	}, "2025", rules)
	c := findCategory(t, overview, "insurance", "health_insurance")
	require.Len(t, c.Postings, 1)
	assert.Equal(t, "account", c.Postings[0].Source)
}

func TestBuildTaxOverviewLegacyTagIsIgnored(t *testing.T) {
	overview := BuildTaxOverview([]posting.Posting{
		taxOverviewPosting("t1", "2025-05-01", "Expenses:Household:Goods", "100", "", "Work"),
	}, "2025", nil)
	assert.True(t, overview.Summary.ExpenseCandidates.IsZero())
	assert.Equal(t, 1, overview.Coverage.UnmatchedPostingCount)
	assert.Contains(t, diagnosticSummaries(overview), "TaxRelevant")
}

func TestBuildTaxOverviewLegacyTagDoesNotOverrideAccountRule(t *testing.T) {
	rules := taxRules("Expenses:Health", "health")
	overview := BuildTaxOverview([]posting.Posting{
		taxOverviewPosting("t1", "2025-05-01", "Expenses:Health", "100", "", "Work"),
	}, "2025", rules)
	c := findCategory(t, overview, "extraordinary", "health")
	require.Len(t, c.Postings, 1)
	assert.Equal(t, "account", c.Postings[0].Source)
	assert.Contains(t, diagnosticSummaries(overview), "TaxRelevant")
}

func TestBuildTaxOverviewNegativePostingReducesTotal(t *testing.T) {
	postings := []posting.Posting{
		taxOverviewPosting("t1", "2025-05-01", "Expenses:Health", "100", "health", ""),
		taxOverviewPosting("t2", "2025-06-01", "Expenses:Health", "-45", "health", ""),
	}
	overview := BuildTaxOverview(postings, "2025", nil)
	c := findCategory(t, overview, "extraordinary", "health")
	assert.True(t, c.Total.Equal(decimal.RequireFromString("55")))
	assert.Contains(t, diagnosticSummaries(overview), "Negative Aufwandsposten")
}

func TestBuildTaxOverviewCoverageGroupsByAccount(t *testing.T) {
	postings := []posting.Posting{
		taxOverviewPosting("t1", "2025-05-01", "Expenses:Household:Goods", "10", "", ""),
		taxOverviewPosting("t2", "2025-05-02", "Expenses:Household:Goods", "20", "", ""),
		taxOverviewPosting("t3", "2025-05-03", "Expenses:Food", "30", "", ""),
	}
	overview := BuildTaxOverview(postings, "2025", nil)
	assert.Equal(t, 3, overview.Coverage.UnmatchedPostingCount)
	assert.Equal(t, 2, overview.Coverage.AccountsWithUnmatchedPostings)
	require.Len(t, overview.Coverage.Accounts, 2)
	assert.Equal(t, "Expenses:Household:Goods", overview.Coverage.Accounts[0].Account)
	assert.Equal(t, 2, overview.Coverage.Accounts[0].UnmatchedPostingCount)
	assert.Equal(t, "Expenses:Food", overview.Coverage.Accounts[1].Account)
}

func TestBuildTaxOverviewCoverageSameAccountTwice(t *testing.T) {
	postings := []posting.Posting{
		taxOverviewPosting("t1", "2025-05-01", "Expenses:Household:Goods", "10", "", ""),
		taxOverviewPosting("t2", "2025-05-02", "Expenses:Household:Goods", "20", "", ""),
	}
	overview := BuildTaxOverview(postings, "2025", nil)
	assert.Equal(t, 2, overview.Coverage.UnmatchedPostingCount)
	assert.Equal(t, 1, overview.Coverage.AccountsWithUnmatchedPostings)
}

func TestBuildTaxOverviewZeroAmountIsMatchedAndDiagnosed(t *testing.T) {
	overview := BuildTaxOverview([]posting.Posting{
		taxOverviewPosting("t1", "2025-05-01", "Expenses:Health", "0", "health", ""),
	}, "2025", nil)
	c := findCategory(t, overview, "extraordinary", "health")
	require.Len(t, c.Postings, 1)
	assert.True(t, c.Postings[0].Amount.IsZero())
	assert.Equal(t, 1, overview.Summary.MatchedExpensePostings)
	assert.Equal(t, 0, overview.Coverage.UnmatchedPostingCount)
	assert.Contains(t, diagnosticSummaries(overview), "Betrag von 0")
}

func TestBuildTaxOverviewEmptySections(t *testing.T) {
	overview := BuildTaxOverview(nil, "2025", nil)
	require.Len(t, overview.Sections, 5)
	for _, section := range overview.Sections {
		assert.NotEqual(t, "capital_income", section.Key)
		assert.NotNil(t, section.Categories)
		for _, category := range section.Categories {
			assert.NotNil(t, category.Postings)
		}
	}
	assert.NotNil(t, overview.Diagnostics)
	assert.NotNil(t, overview.Coverage.Accounts)
	assert.NotNil(t, overview.CapitalIncome.Accounts)
}

func TestBuildTaxOverviewSummaryMatchesSections(t *testing.T) {
	postings := []posting.Posting{
		taxOverviewPosting("t1", "2025-05-01", "Expenses:Health", "100", "health", ""),
		taxOverviewPosting("t2", "2025-05-02", "Expenses:Household:Goods", "699", "work_equipment", ""),
	}
	overview := BuildTaxOverview(postings, "2025", nil)
	total := decimal.Zero
	count := 0
	for _, section := range overview.Sections {
		total = total.Add(section.Total)
		count += section.PostingCount
	}
	assert.True(t, overview.Summary.ExpenseCandidates.Equal(total))
	assert.Equal(t, count, overview.Summary.MatchedExpensePostings)
	assert.True(t, overview.Summary.ExpenseCandidates.Equal(decimal.RequireFromString("799")))
	assert.Equal(t, 2, overview.Summary.MatchedExpensePostings)
}

func TestTaxOverviewYears(t *testing.T) {
	now := time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)
	postings := []posting.Posting{
		taxOverviewPosting("t1", "2025-05-01", "Expenses:Health", "1", "", ""),
		taxOverviewPosting("t2", "2024-05-01", "Expenses:Health", "1", "", ""),
		taxOverviewPosting("t3", "2024-06-01", "Expenses:Health", "1", "", ""),
		taxOverviewPosting("t4", "0001-01-01", "Expenses:Health", "1", "", ""),
		taxOverviewPosting("t5", "1899-05-01", "Expenses:Health", "1", "", ""),
		{TransactionID: "t6", Account: "Expenses:Health"},
	}
	assert.Equal(t, []string{"2024", "2025"}, TaxOverviewYears(postings, now))
}

func TestResolveTaxPostingStatuses(t *testing.T) {
	rules := taxRules("Expenses:Health", "health")
	now := taxOverviewPosting("t1", "2025-05-01", "Expenses:Health", "100", "", "")

	matched := resolveTaxPosting(taxOverviewPosting("t1", "2025-05-01", "Expenses:Health", "100", "health", ""), []posting.Posting{now}, rules)
	assert.Equal(t, taxMatched, matched.Status)
	assert.Equal(t, "health", matched.Category)
	assert.Equal(t, "tag", matched.Source)

	excluded := resolveTaxPosting(taxOverviewPosting("t1", "2025-05-01", "Expenses:Health", "100", "none", ""), []posting.Posting{now}, rules)
	assert.Equal(t, taxExcluded, excluded.Status)

	unresolved := resolveTaxPosting(taxOverviewPosting("t1", "2025-05-01", "Expenses:Food", "100", "", ""), []posting.Posting{now}, rules)
	assert.Equal(t, taxUnresolved, unresolved.Status)

	invalid := resolveTaxPosting(taxOverviewPosting("t1", "2025-05-01", "Expenses:Health", "100", "made_up", ""), []posting.Posting{now}, rules)
	assert.Equal(t, taxInvalid, invalid.Status)
	require.NotNil(t, invalid.Diagnostic)
	assert.Contains(t, invalid.Diagnostic.Summary, "Unbekannte Steuerkategorie")

	ambiguous := resolveTaxPosting(
		taxOverviewPosting("t1", "2025-05-01", "Expenses:Health", "100", "", ""),
		[]posting.Posting{
			taxOverviewPosting("t1", "2025-05-01", "Assets:Checking", "-100", "health", ""),
			taxOverviewPosting("t1", "2025-05-01", "Expenses:Health", "60", "", ""),
			taxOverviewPosting("t1", "2025-05-01", "Expenses:Health:Medicine", "40", "", ""),
		},
		nil)
	assert.Equal(t, taxInvalid, ambiguous.Status)
	require.NotNil(t, ambiguous.Diagnostic)
	assert.Contains(t, ambiguous.Diagnostic.Summary, "nicht genau einen Aufwandsposten")
}

func TestBuildTaxOverviewIgnoresOtherYearDiagnostics(t *testing.T) {
	postings := []posting.Posting{
		taxOverviewPosting("t1", "2024-05-01", "Expenses:Health", "-45", "health", ""),
		taxOverviewPosting("t2", "2024-06-01", "Expenses:Household:Goods", "100", "", "Work"),
		taxOverviewPosting("t3", "2025-05-01", "Expenses:Health", "100", "health", ""),
	}
	overview := BuildTaxOverview(postings, "2025", nil)
	assert.NotContains(t, diagnosticSummaries(overview), "Negative Aufwandsposten")
	assert.NotContains(t, diagnosticSummaries(overview), "TaxRelevant")
	assert.Equal(t, 0, overview.Coverage.UnmatchedPostingCount)
}

func TestBuildTaxOverviewTaggedTransactionWithoutExpenseIsDiagnosed(t *testing.T) {
	postings := []posting.Posting{
		taxOverviewPosting("t1", "2025-05-01", "Assets:Checking", "-1000", "work_equipment", ""),
		taxOverviewPosting("t1", "2025-05-01", "Assets:Equity", "1000", "", ""),
	}
	overview := BuildTaxOverview(postings, "2025", nil)
	assert.Contains(t, diagnosticSummaries(overview), "nicht genau einen Aufwandsposten")
	assert.Equal(t, 1, len(overview.Diagnostics))
}
