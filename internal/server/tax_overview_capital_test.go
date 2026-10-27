package server

import (
	"testing"

	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCapitalIncomeSplitsGainsAndLossesPerPair(t *testing.T) {
	postings := []posting.Posting{
		germanyTaxPosting("2025-01-01", "Assets:Equity:VWCE", "VWCE", "10", "1000"),
		func() posting.Posting {
			p := germanyTaxPosting("2025-02-01", "Assets:Equity:VWCE", "VWCE", "-2", "-300")
			p.TransactionID = "gain"
			return p
		}(),
		func() posting.Posting {
			p := germanyTaxPosting("2025-03-01", "Assets:Equity:VWCE", "VWCE", "-2", "-100")
			p.TransactionID = "loss"
			return p
		}(),
	}

	income, diagnostics := ComputeTaxOverviewCapitalIncome(postings, nil, nil, "2025")

	assert.True(t, income.RealizedGains.Equal(decimal.RequireFromString("100")))
	assert.True(t, income.RealizedLosses.Equal(decimal.RequireFromString("100")))
	assert.Equal(t, 2, income.Sales)
	assert.Empty(t, diagnostics)
	require.Len(t, income.Accounts, 1)
	assert.Equal(t, "Assets:Equity:VWCE", income.Accounts[0].Account)
	assert.True(t, income.Accounts[0].RealizedGain.IsZero())
}

func TestCapitalIncomeIncludesDividendsOnlyYear(t *testing.T) {
	incomePostings := []posting.Posting{
		germanyTaxPosting("2025-06-01", "Income:Dividend:VWCE", "", "0", "-118.06"),
	}
	income, diagnostics := ComputeTaxOverviewCapitalIncome(nil, incomePostings, nil, "2025")
	assert.True(t, income.Dividends.Equal(decimal.RequireFromString("118.06")))
	assert.Equal(t, 0, income.Sales)
	assert.Empty(t, income.Accounts)
	require.Len(t, diagnostics, 1)
	assert.Contains(t, diagnostics[0].Summary, "Dividendeneingänge ohne gebuchte Steuern")
}

func TestCapitalIncomeCountsDistinctSalesAndTaxes(t *testing.T) {
	postings := []posting.Posting{
		germanyTaxPosting("2025-01-01", "Assets:Equity:VWCE", "VWCE", "10", "1000"),
		func() posting.Posting {
			p := germanyTaxPosting("2025-02-01", "Assets:Equity:VWCE", "VWCE", "-2", "-300")
			p.TransactionID = "sale"
			return p
		}(),
		func() posting.Posting {
			p := germanyTaxPosting("2025-03-01", "Assets:Equity:VWCE", "VWCE", "-2", "-300")
			p.TransactionID = "sale"
			return p
		}(),
	}
	incomePostings := []posting.Posting{
		germanyTaxPosting("2025-06-01", "Income:Dividend:VWCE", "", "0", "-100"),
	}
	withholding := []posting.Posting{
		germanyTaxPosting("2025-06-01", "Expenses:Broker:Taxes", "", "0", "10"),
		germanyTaxPosting("2025-06-02", "Expenses:Broker:Taxes", "", "0", "5"),
	}

	result, diagnostics := ComputeTaxOverviewCapitalIncome(postings, incomePostings, withholding, "2025")
	assert.Equal(t, 1, result.Sales)
	assert.True(t, result.TaxesBooked.Equal(decimal.RequireFromString("15")))
	assert.Empty(t, diagnostics)
}

func TestCapitalIncomeCountsEmptyTransactionIDsAsSeparateSales(t *testing.T) {
	postings := []posting.Posting{
		germanyTaxPosting("2025-01-01", "Assets:Equity:VWCE", "VWCE", "10", "1000"),
		germanyTaxPosting("2025-02-01", "Assets:Equity:VWCE", "VWCE", "-2", "-300"),
		germanyTaxPosting("2025-03-01", "Assets:Equity:VWCE", "VWCE", "-2", "-300"),
	}
	result, _ := ComputeTaxOverviewCapitalIncome(postings, nil, nil, "2025")
	assert.Equal(t, 2, result.Sales)
}

func TestCapitalIncomeDoesNotGrossUpDividends(t *testing.T) {
	incomePostings := []posting.Posting{
		germanyTaxPosting("2025-06-01", "Income:Dividend:VWCE", "", "0", "-100"),
	}
	withholding := []posting.Posting{
		germanyTaxPosting("2025-06-01", "Expenses:Broker:Taxes", "", "0", "25"),
	}
	result, _ := ComputeTaxOverviewCapitalIncome(nil, incomePostings, withholding, "2025")
	assert.True(t, result.Dividends.Equal(decimal.RequireFromString("100")))
	assert.True(t, result.TaxesBooked.Equal(decimal.RequireFromString("25")))
}

func TestCapitalIncomeWarnsWhenIncomeHasNoBookedTaxes(t *testing.T) {
	incomePostings := []posting.Posting{
		germanyTaxPosting("2025-06-01", "Income:Dividend:VWCE", "", "0", "-100"),
	}
	_, diagnostics := ComputeTaxOverviewCapitalIncome(nil, incomePostings, nil, "2025")
	require.Len(t, diagnostics, 1)
	assert.Contains(t, diagnostics[0].Summary, "Dividendeneingänge ohne gebuchte Steuern")
}

func TestCapitalIncomeIgnoresOtherYears(t *testing.T) {
	incomePostings := []posting.Posting{
		germanyTaxPosting("2024-06-01", "Income:Dividend:VWCE", "", "0", "-50"),
		germanyTaxPosting("2025-06-01", "Income:Dividend:VWCE", "", "0", "-100"),
	}
	result, diagnostics := ComputeTaxOverviewCapitalIncome(nil, incomePostings, nil, "2025")
	assert.True(t, result.Dividends.Equal(decimal.RequireFromString("100")))
	require.Len(t, diagnostics, 1)
}

func TestCapitalIncomeWarnsAboutSalesWithoutPurchases(t *testing.T) {
	postings := []posting.Posting{
		func() posting.Posting {
			p := germanyTaxPosting("2025-02-01", "Assets:Equity:VWCE", "VWCE", "-2", "-300")
			p.TransactionID = "orphan"
			return p
		}(),
	}
	result, diagnostics := ComputeTaxOverviewCapitalIncome(postings, nil, nil, "2025")
	assert.True(t, result.RealizedGains.IsZero())
	assert.True(t, result.RealizedLosses.IsZero())
	require.Len(t, diagnostics, 1)
	assert.Contains(t, diagnostics[0].Summary, "ohne passende Anschaffung")
}

func TestCapitalIncomeWarnsAboutPartiallyMatchedSales(t *testing.T) {
	postings := []posting.Posting{
		germanyTaxPosting("2025-01-01", "Assets:Equity:VWCE", "VWCE", "1", "100"),
		func() posting.Posting {
			p := germanyTaxPosting("2025-02-01", "Assets:Equity:VWCE", "VWCE", "-2", "-300")
			p.TransactionID = "partial"
			return p
		}(),
	}
	result, diagnostics := ComputeTaxOverviewCapitalIncome(postings, nil, nil, "2025")
	assert.True(t, result.RealizedGains.Equal(decimal.RequireFromString("50")))
	require.Len(t, diagnostics, 1)
	assert.Contains(t, diagnostics[0].Summary, "ohne passende Anschaffung")
}
