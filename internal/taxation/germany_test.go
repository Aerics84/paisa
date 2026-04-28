package taxation

import (
	"testing"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestCalculateGermanyTaxUsesAllowanceAndRates(t *testing.T) {
	breakdown := CalculateGermanyTax(decimal.NewFromInt(2500), config.GermanyTaxConfig{
		AnnualAllowance:         1000,
		CapitalIncomeTaxRate:    0.25,
		SolidaritySurchargeRate: 0.055,
		ChurchTaxRate:           0.09,
	})

	assert.True(t, breakdown.RealizedGain.Equal(decimal.NewFromInt(2500)))
	assert.True(t, breakdown.GrossRealizedGain.Equal(decimal.NewFromInt(2500)))
	assert.True(t, breakdown.RealizedLoss.Equal(decimal.Zero))
	assert.True(t, breakdown.AllowanceUsed.Equal(decimal.NewFromInt(1000)))
	assert.True(t, breakdown.TaxableAmountBeforeAllowance.Equal(decimal.NewFromInt(2500)))
	assert.True(t, breakdown.TaxableAmount.Equal(decimal.NewFromInt(1500)))
	assert.True(t, breakdown.CapitalIncomeTax.Equal(decimal.NewFromInt(375)))
	assert.True(t, breakdown.SolidaritySurcharge.Equal(decimal.NewFromFloat(20.625)))
	assert.True(t, breakdown.ChurchTax.Equal(decimal.NewFromFloat(33.75)))
	assert.True(t, breakdown.TotalTax.Equal(decimal.NewFromFloat(429.375)))
	assert.True(t, breakdown.NetTaxDue.Equal(decimal.NewFromFloat(429.375)))
}

func TestCalculateGermanyTaxDoesNotTaxLosses(t *testing.T) {
	breakdown := CalculateGermanyTax(decimal.NewFromInt(-50), config.GermanyTaxConfig{
		AnnualAllowance:         1000,
		CapitalIncomeTaxRate:    0.25,
		SolidaritySurchargeRate: 0.055,
		ChurchTaxRate:           0.09,
	})

	assert.True(t, breakdown.RealizedGain.Equal(decimal.NewFromInt(-50)))
	assert.True(t, breakdown.RealizedLoss.Equal(decimal.NewFromInt(50)))
	assert.True(t, breakdown.AllowanceUsed.Equal(decimal.Zero))
	assert.True(t, breakdown.TaxableAmount.Equal(decimal.Zero))
	assert.True(t, breakdown.TotalTax.Equal(decimal.Zero))
	assert.True(t, breakdown.NetTaxDue.Equal(decimal.Zero))
}

func TestCalculateGermanyTaxDetailedAppliesCreditsAndAdjustments(t *testing.T) {
	breakdown := CalculateGermanyTaxDetailed(GermanyTaxInput{
		GrossRealizedGain:            decimal.NewFromInt(2000),
		RealizedLoss:                 decimal.NewFromInt(500),
		RealizedGain:                 decimal.NewFromInt(1500),
		PartialExemptionAmount:       decimal.NewFromInt(300),
		TaxableAmountBeforeAllowance: decimal.NewFromInt(1200),
		WithholdingTaxPaid:           decimal.NewFromInt(100),
	}, config.GermanyTaxConfig{
		AnnualAllowance:         1000,
		CapitalIncomeTaxRate:    0.25,
		SolidaritySurchargeRate: 0.055,
		ChurchTaxRate:           0.09,
	})

	assert.True(t, breakdown.PartialExemptionAmount.Equal(decimal.NewFromInt(300)))
	assert.True(t, breakdown.AllowanceUsed.Equal(decimal.NewFromInt(1000)))
	assert.True(t, breakdown.TaxableAmount.Equal(decimal.NewFromInt(200)))
	assert.True(t, breakdown.TotalTax.Equal(decimal.NewFromFloat(57.25)))
	assert.True(t, breakdown.TaxCreditUsed.Equal(decimal.NewFromFloat(57.25)))
	assert.True(t, breakdown.NetTaxDue.Equal(decimal.Zero))
}
