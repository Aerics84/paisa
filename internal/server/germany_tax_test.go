package server

import (
	"testing"
	"time"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func germanyTaxPosting(date string, account string, commodity string, quantity string, amount string) posting.Posting {
	parsed, err := time.Parse("2006-01-02", date)
	if err != nil {
		panic(err)
	}

	return posting.Posting{
		Date:      parsed,
		Account:   account,
		Commodity: commodity,
		Quantity:  decimal.RequireFromString(quantity),
		Amount:    decimal.RequireFromString(amount),
	}
}

func TestComputeGermanyTaxByYearUsesFIFOGains(t *testing.T) {
	postings := []posting.Posting{
		germanyTaxPosting("2024-01-10", "Assets:Equity:VWCE", "VWCE", "10", "1000"),
		germanyTaxPosting("2024-02-15", "Assets:Equity:VWCE", "VWCE", "-4", "-520"),
		germanyTaxPosting("2024-03-01", "Assets:Equity:VWCE", "VWCE", "2", "260"),
		germanyTaxPosting("2025-01-05", "Assets:Equity:VWCE", "VWCE", "-3", "-390"),
	}

	byYear := computeGermanyTaxByYear("Assets:Equity:VWCE", postings)

	assert.Len(t, byYear, 2)
	assert.Equal(t, "Assets:Equity:VWCE", byYear["2024"].Account)
	assert.True(t, byYear["2024"].Units.Equal(decimal.RequireFromString("4")))
	assert.True(t, byYear["2024"].PurchasePrice.Equal(decimal.RequireFromString("400")))
	assert.True(t, byYear["2024"].SellPrice.Equal(decimal.RequireFromString("520")))
	assert.True(t, byYear["2024"].RealizedGain.Equal(decimal.RequireFromString("120")))
	assert.True(t, byYear["2024"].TaxableGain.Equal(decimal.RequireFromString("120")))

	assert.True(t, byYear["2025"].Units.Equal(decimal.RequireFromString("3")))
	assert.True(t, byYear["2025"].PurchasePrice.Equal(decimal.RequireFromString("300")))
	assert.True(t, byYear["2025"].SellPrice.Equal(decimal.RequireFromString("390")))
	assert.True(t, byYear["2025"].RealizedGain.Equal(decimal.RequireFromString("90")))
}

func TestComputeGermanyTaxByYearAppliesConfiguredPartialExemption(t *testing.T) {
	err := config.LoadConfig([]byte(`
journal_path: main.ledger
db_path: paisa.db
commodities:
  - name: VWCE
    type: mutualfund
    price:
      provider: com-yahoo
      code: VWCE
    germany_partial_exemption_rate: 0.3
`), "")
	require.NoError(t, err)

	postings := []posting.Posting{
		germanyTaxPosting("2024-01-10", "Assets:Equity:VWCE", "VWCE", "10", "1000"),
		germanyTaxPosting("2024-02-15", "Assets:Equity:VWCE", "VWCE", "-4", "-520"),
	}

	byYear := computeGermanyTaxByYear("Assets:Equity:VWCE", postings)

	assert.True(t, byYear["2024"].RealizedGain.Equal(decimal.RequireFromString("120")))
	assert.True(t, byYear["2024"].PartialExemptionAmount.Equal(decimal.RequireFromString("36")))
	assert.True(t, byYear["2024"].TaxableGain.Equal(decimal.RequireFromString("84")))
	if assert.NotNil(t, byYear["2024"].PartialExemptionRate) {
		assert.Equal(t, 0.3, *byYear["2024"].PartialExemptionRate)
	}
}

func TestBuildGermanyTaxYearAddsDiagnosticsAndWithholdingCredits(t *testing.T) {
	rate := 0.3
	accounts := []GermanyTaxAccount{
		{
			Account:                "Assets:Equity:VWCE",
			Commodity:              "VWCE",
			CommodityType:          config.MutualFund,
			PartialExemptionRate:   &rate,
			RealizedGain:           decimal.RequireFromString("120"),
			PartialExemptionAmount: decimal.RequireFromString("36"),
			TaxableGain:            decimal.RequireFromString("84"),
		},
		{
			Account:       "Assets:Equity:MSCI",
			Commodity:     "MSCI",
			CommodityType: config.MutualFund,
			RealizedGain:  decimal.RequireFromString("80"),
			TaxableGain:   decimal.RequireFromString("80"),
		},
		{
			Account:       "Assets:Equity:AAPL",
			Commodity:     "AAPL",
			CommodityType: config.Stock,
			RealizedGain:  decimal.RequireFromString("-50"),
			TaxableGain:   decimal.RequireFromString("-50"),
		},
	}
	incomePostings := []posting.Posting{
		germanyTaxPosting("2024-03-01", "Income:Dividend:VWCE", "", "0", "45.67"),
	}
	withholding := []posting.Posting{
		germanyTaxPosting("2024-03-01", "Expenses:Broker:Taxes", "", "0", "30"),
	}

	report := buildGermanyTaxYear("2024", config.GermanyTaxConfig{
		AnnualAllowance:         100,
		CapitalIncomeTaxRate:    0.25,
		SolidaritySurchargeRate: 0.055,
		ChurchTaxRate:           0,
	}, accounts, incomePostings, withholding)

	assert.True(t, report.Summary.GrossRealizedGain.Equal(decimal.RequireFromString("200")))
	assert.True(t, report.Summary.RealizedLoss.Equal(decimal.RequireFromString("50")))
	assert.True(t, report.Summary.RealizedGain.Equal(decimal.RequireFromString("150")))
	assert.True(t, report.Summary.PartialExemptionAmount.Equal(decimal.RequireFromString("36")))
	assert.True(t, report.Summary.TaxableAmountBeforeAllowance.Equal(decimal.RequireFromString("114")))
	assert.True(t, report.Summary.WithholdingTaxPaid.Equal(decimal.RequireFromString("30")))
	assert.True(t, report.Summary.TaxCreditUsed.Equal(decimal.RequireFromString("3.6925")))
	assert.True(t, report.Summary.NetTaxDue.Equal(decimal.Zero))
	assert.Len(t, report.Diagnostics, 1)
	assert.Contains(t, report.Diagnostics[0].Summary, "Missing ETF partial-exemption metadata")
}
