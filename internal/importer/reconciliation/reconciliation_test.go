package reconciliation

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReconcileBankBalanceDirection(t *testing.T) {
	report := BuildReconciliation(ReconciliationInput{
		BankStatements: []BankStatement{
			{
				Source: "statements/ascending.csv",
				Transactions: []BankTransaction{
					{Date: "2026-01-01", Credit: "100.00", Balance: "1100.00", Currency: "INR"},
					{Date: "2026-01-02", Debit: "50.00", Balance: "1050.00", Currency: "INR"},
					{Date: "2026-01-03", Credit: "25.50", Balance: "1075.50", Currency: "INR"},
				},
			},
			{
				Source: "statements/descending.csv",
				Transactions: []BankTransaction{
					{Date: "2026-01-03", Credit: "25.50", Balance: "1075.50", Currency: "INR"},
					{Date: "2026-01-02", Debit: "50.00", Balance: "1050.00", Currency: "INR"},
					{Date: "2026-01-01", Credit: "100.00", Balance: "1100.00", Currency: "INR"},
				},
			},
			{
				Source: "statements/single.csv",
				Transactions: []BankTransaction{
					{Date: "2026-01-01", Credit: "10.00", Balance: "110.00", Currency: "INR"},
				},
			},
		},
	})

	require.Len(t, report.BankBalances, 3)

	ascending := report.BankBalances[0]
	assert.Equal(t, "statements/ascending.csv", ascending.Source)
	assert.Equal(t, "INR", ascending.Currency)
	assert.Equal(t, 3, ascending.TransactionCount)
	assert.Equal(t, "ascending", ascending.Direction)
	require.NotNil(t, ascending.OpeningBalance)
	require.NotNil(t, ascending.ClosingBalance)
	assert.Equal(t, "1000.00", *ascending.OpeningBalance)
	assert.Equal(t, "1075.50", *ascending.ClosingBalance)
	assert.Equal(t, 0, ascending.MismatchCount)
	assert.Equal(t, "ok", ascending.Status)

	descending := report.BankBalances[1]
	assert.Equal(t, "descending", descending.Direction)
	require.NotNil(t, descending.OpeningBalance)
	require.NotNil(t, descending.ClosingBalance)
	assert.Equal(t, "1000.00", *descending.OpeningBalance)
	assert.Equal(t, "1075.50", *descending.ClosingBalance)
	assert.Equal(t, 0, descending.MismatchCount)
	assert.Equal(t, "ok", descending.Status)

	single := report.BankBalances[2]
	assert.Equal(t, "unknown", single.Direction)
	require.NotNil(t, single.OpeningBalance)
	require.NotNil(t, single.ClosingBalance)
	assert.Equal(t, "100.00", *single.OpeningBalance)
	assert.Equal(t, "110.00", *single.ClosingBalance)
	assert.Equal(t, "ok", single.Status)

	assert.Equal(t, "ok", report.Status)
	assert.Empty(t, report.Issues)
}

func TestReconcileBankWithoutBalanceIsSkipped(t *testing.T) {
	report := BuildReconciliation(ReconciliationInput{
		BankStatements: []BankStatement{
			{
				Source: "Umsatzanzeige.csv",
				Transactions: []BankTransaction{
					{Date: "2026-11-02", Debit: "10.00", Currency: "EUR"},
				},
			},
		},
	})

	require.Len(t, report.BankBalances, 1)
	check := report.BankBalances[0]
	assert.Equal(t, "skipped", check.Status)
	assert.Equal(t, "unavailable", check.Direction)
	assert.Equal(t, "EUR", check.Currency)
	assert.Nil(t, check.OpeningBalance)
	assert.Nil(t, check.ClosingBalance)
	assert.Equal(t, "incomplete", report.Status)
	require.Len(t, report.Issues, 1)
	assert.Contains(t, report.Issues[0], "no balance column")
}

func TestReconcileBankMismatch(t *testing.T) {
	report := BuildReconciliation(ReconciliationInput{
		BankStatements: []BankStatement{
			{
				Source: "statements/broken.csv",
				Transactions: []BankTransaction{
					{Date: "2026-01-01", Credit: "100.00", Balance: "1100.00", Currency: "INR"},
					{Date: "2026-01-02", Debit: "50.00", Balance: "1050.00", Currency: "INR"},
					// 1050 + 25.50 should be 1075.50, not 1000.00.
					{Date: "2026-01-03", Credit: "25.50", Balance: "1000.00", Currency: "INR"},
				},
			},
		},
	})

	require.Len(t, report.BankBalances, 1)
	check := report.BankBalances[0]
	assert.Equal(t, "ascending", check.Direction)
	assert.Equal(t, 1, check.MismatchCount)
	assert.Equal(t, "error", check.Status)
	assert.Equal(t, "error", report.Status)
}

func TestReconcileDuplicateHoldings(t *testing.T) {
	report := BuildReconciliation(ReconciliationInput{
		Holdings: []Holding{
			{
				Security:     Security{ISIN: "INE009A01021"},
				CurrentValue: "1000.00",
				Currency:     "INR",
				Source:       HoldingSource{OriginalPath: "holdings/zerodha.csv"},
			},
			{
				Security:     Security{ISIN: "ine009a01021"},
				CurrentValue: "1000.00",
				Currency:     "INR",
				Source:       HoldingSource{OriginalPath: "holdings/cas.csv"},
			},
			{
				Security:     Security{Symbol: "TCS"},
				CurrentValue: "500.00",
				Currency:     "INR",
				Source:       HoldingSource{OriginalPath: "holdings/zerodha.csv"},
			},
		},
	})

	require.Len(t, report.DuplicateHoldings, 1)
	duplicate := report.DuplicateHoldings[0]
	assert.Equal(t, "INE009A01021", duplicate.Key)
	assert.Equal(t, 2, duplicate.Occurrences)
	assert.Equal(t, []string{"holdings/cas.csv", "holdings/zerodha.csv"}, duplicate.Sources)

	assert.Contains(t, report.Issues, "1 holding identifier(s) appear more than once.")
	assert.Equal(t, "warning", report.Status)
}

func TestReconcilePortfolioTotals(t *testing.T) {
	report := BuildReconciliation(ReconciliationInput{
		Holdings: []Holding{
			{CurrentValue: "1000.50", Currency: "INR"},
			{CurrentValue: "250", Currency: "INR"},
			{CurrentValue: "not-a-number", Currency: "INR"},
			{CurrentValue: "100", Currency: "USD"},
		},
		Balances: []ManualBalance{
			{Amount: "500", Currency: "INR", BalanceType: "asset"},
			{Amount: "-200", Currency: "INR", BalanceType: "liability"},
			{Amount: "999", Currency: "INR", BalanceType: "cash_flow"},
			{Amount: "bad", Currency: "INR", BalanceType: "asset"},
		},
	})

	require.Len(t, report.PortfolioTotals, 2)

	totals := map[string]CurrencyTotal{}
	for _, total := range report.PortfolioTotals {
		totals[total.Currency] = total
	}

	inr := totals["INR"]
	assert.Equal(t, "1250.50", inr.Holdings)
	assert.Equal(t, "500.00", inr.Assets)
	assert.Equal(t, "200.00", inr.Liabilities)
	assert.Equal(t, "1550.50", inr.Net)
	assert.Equal(t, 2, inr.InvalidRecords)

	usd := totals["USD"]
	assert.Equal(t, "100.00", usd.Holdings)
	assert.Equal(t, "0.00", usd.Assets)
	assert.Equal(t, "0.00", usd.Liabilities)
	assert.Equal(t, "100.00", usd.Net)
	assert.Equal(t, 0, usd.InvalidRecords)

	assert.Contains(t, report.Issues, "Some portfolio records contain invalid amounts.")
	assert.Equal(t, "error", report.Status)
}

func TestReconcileTaxYearCompleteness(t *testing.T) {
	report := BuildReconciliation(ReconciliationInput{
		TaxRecords: []TaxRecord{
			{Source: "itr", FinancialYear: "2024-25"},
			{Source: "ais", FinancialYear: "2024-25"},
			{Source: "form26as", FinancialYear: "2024-25"},
			{Source: "itr", FinancialYear: "2025-26"},
			{Source: "annual_tax_statement", FinancialYear: "2023-24"},
		},
	})

	require.Len(t, report.TaxYearCompleteness, 3)

	assert.Equal(t, "2023-24", report.TaxYearCompleteness[0].FinancialYear)
	assert.Equal(t, []string{"annual_tax_statement"}, report.TaxYearCompleteness[0].SourcesPresent)
	assert.Equal(t, []string{"itr", "ais"}, report.TaxYearCompleteness[0].MissingRequired)
	assert.Equal(t, "incomplete", report.TaxYearCompleteness[0].Status)

	assert.Equal(t, "2024-25", report.TaxYearCompleteness[1].FinancialYear)
	assert.Equal(t, []string{"ais", "form26as", "itr"}, report.TaxYearCompleteness[1].SourcesPresent)
	assert.Empty(t, report.TaxYearCompleteness[1].MissingRequired)
	assert.Equal(t, "complete", report.TaxYearCompleteness[1].Status)

	assert.Equal(t, "2025-26", report.TaxYearCompleteness[2].FinancialYear)
	assert.Equal(t, []string{"ais", "tax_statement"}, report.TaxYearCompleteness[2].MissingRequired)
	assert.Equal(t, "incomplete", report.TaxYearCompleteness[2].Status)

	assert.Contains(t, report.Issues, "One or more tax years are missing a required source family.")
	assert.Equal(t, "warning", report.Status)
}

func TestReconcileEmptyInput(t *testing.T) {
	report := BuildReconciliation(ReconciliationInput{})

	assert.Equal(t, "ok", report.Status)
	assert.NotEmpty(t, report.GeneratedAt)
	_, err := time.Parse(time.RFC3339, report.GeneratedAt)
	require.NoError(t, err)

	assert.NotNil(t, report.BankBalances)
	assert.NotNil(t, report.DuplicateHoldings)
	assert.NotNil(t, report.PortfolioTotals)
	assert.NotNil(t, report.TaxYearCompleteness)
	assert.NotNil(t, report.Issues)
	assert.Empty(t, report.BankBalances)
	assert.Empty(t, report.DuplicateHoldings)
	assert.Empty(t, report.PortfolioTotals)
	assert.Empty(t, report.TaxYearCompleteness)
	assert.Empty(t, report.Issues)
}
