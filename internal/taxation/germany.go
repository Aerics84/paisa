package taxation

import (
	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/shopspring/decimal"
)

type GermanyTaxBreakdown struct {
	RealizedGain        decimal.Decimal `json:"realized_gain"`
	AllowanceUsed       decimal.Decimal `json:"allowance_used"`
	TaxableAmount       decimal.Decimal `json:"taxable_amount"`
	CapitalIncomeTax    decimal.Decimal `json:"capital_income_tax"`
	SolidaritySurcharge decimal.Decimal `json:"solidarity_surcharge"`
	ChurchTax           decimal.Decimal `json:"church_tax"`
	TotalTax            decimal.Decimal `json:"total_tax"`
}

func AddGermanyBreakdown(a, b GermanyTaxBreakdown) GermanyTaxBreakdown {
	return GermanyTaxBreakdown{
		RealizedGain:        a.RealizedGain.Add(b.RealizedGain),
		AllowanceUsed:       a.AllowanceUsed.Add(b.AllowanceUsed),
		TaxableAmount:       a.TaxableAmount.Add(b.TaxableAmount),
		CapitalIncomeTax:    a.CapitalIncomeTax.Add(b.CapitalIncomeTax),
		SolidaritySurcharge: a.SolidaritySurcharge.Add(b.SolidaritySurcharge),
		ChurchTax:           a.ChurchTax.Add(b.ChurchTax),
		TotalTax:            a.TotalTax.Add(b.TotalTax),
	}
}

func CalculateGermanyTax(realizedGain decimal.Decimal, settings config.GermanyTaxConfig) GermanyTaxBreakdown {
	breakdown := GermanyTaxBreakdown{RealizedGain: realizedGain}
	if realizedGain.LessThanOrEqual(decimal.Zero) {
		return breakdown
	}

	allowance := decimal.NewFromFloat(settings.AnnualAllowance)
	breakdown.AllowanceUsed = decimal.Min(realizedGain, allowance)
	breakdown.TaxableAmount = decimal.Max(realizedGain.Sub(breakdown.AllowanceUsed), decimal.Zero)
	breakdown.CapitalIncomeTax = breakdown.TaxableAmount.Mul(decimal.NewFromFloat(settings.CapitalIncomeTaxRate))
	breakdown.SolidaritySurcharge = breakdown.CapitalIncomeTax.Mul(decimal.NewFromFloat(settings.SolidaritySurchargeRate))
	breakdown.ChurchTax = breakdown.CapitalIncomeTax.Mul(decimal.NewFromFloat(settings.ChurchTaxRate))
	breakdown.TotalTax = breakdown.CapitalIncomeTax.Add(breakdown.SolidaritySurcharge).Add(breakdown.ChurchTax)

	return breakdown
}
