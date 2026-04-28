package taxation

import (
	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/shopspring/decimal"
)

type GermanyTaxBreakdown struct {
	GrossRealizedGain            decimal.Decimal `json:"gross_realized_gain"`
	RealizedLoss                 decimal.Decimal `json:"realized_loss"`
	RealizedGain                 decimal.Decimal `json:"realized_gain"`
	PartialExemptionAmount       decimal.Decimal `json:"partial_exemption_amount"`
	TaxableAmountBeforeAllowance decimal.Decimal `json:"taxable_amount_before_allowance"`
	AllowanceUsed                decimal.Decimal `json:"allowance_used"`
	TaxableAmount                decimal.Decimal `json:"taxable_amount"`
	CapitalIncomeTax             decimal.Decimal `json:"capital_income_tax"`
	SolidaritySurcharge          decimal.Decimal `json:"solidarity_surcharge"`
	ChurchTax                    decimal.Decimal `json:"church_tax"`
	WithholdingTaxPaid           decimal.Decimal `json:"withholding_tax_paid"`
	TaxCreditUsed                decimal.Decimal `json:"tax_credit_used"`
	TotalTax                     decimal.Decimal `json:"total_tax"`
	NetTaxDue                    decimal.Decimal `json:"net_tax_due"`
}

type GermanyTaxInput struct {
	GrossRealizedGain            decimal.Decimal
	RealizedLoss                 decimal.Decimal
	RealizedGain                 decimal.Decimal
	PartialExemptionAmount       decimal.Decimal
	TaxableAmountBeforeAllowance decimal.Decimal
	WithholdingTaxPaid           decimal.Decimal
}

func AddGermanyBreakdown(a, b GermanyTaxBreakdown) GermanyTaxBreakdown {
	return GermanyTaxBreakdown{
		GrossRealizedGain:            a.GrossRealizedGain.Add(b.GrossRealizedGain),
		RealizedLoss:                 a.RealizedLoss.Add(b.RealizedLoss),
		RealizedGain:                 a.RealizedGain.Add(b.RealizedGain),
		PartialExemptionAmount:       a.PartialExemptionAmount.Add(b.PartialExemptionAmount),
		TaxableAmountBeforeAllowance: a.TaxableAmountBeforeAllowance.Add(b.TaxableAmountBeforeAllowance),
		AllowanceUsed:                a.AllowanceUsed.Add(b.AllowanceUsed),
		TaxableAmount:                a.TaxableAmount.Add(b.TaxableAmount),
		CapitalIncomeTax:             a.CapitalIncomeTax.Add(b.CapitalIncomeTax),
		SolidaritySurcharge:          a.SolidaritySurcharge.Add(b.SolidaritySurcharge),
		ChurchTax:                    a.ChurchTax.Add(b.ChurchTax),
		WithholdingTaxPaid:           a.WithholdingTaxPaid.Add(b.WithholdingTaxPaid),
		TaxCreditUsed:                a.TaxCreditUsed.Add(b.TaxCreditUsed),
		TotalTax:                     a.TotalTax.Add(b.TotalTax),
		NetTaxDue:                    a.NetTaxDue.Add(b.NetTaxDue),
	}
}

func CalculateGermanyTax(realizedGain decimal.Decimal, settings config.GermanyTaxConfig) GermanyTaxBreakdown {
	input := GermanyTaxInput{
		RealizedGain:                 realizedGain,
		TaxableAmountBeforeAllowance: decimal.Max(realizedGain, decimal.Zero),
	}
	if realizedGain.GreaterThan(decimal.Zero) {
		input.GrossRealizedGain = realizedGain
	} else if realizedGain.LessThan(decimal.Zero) {
		input.RealizedLoss = realizedGain.Neg()
	}
	return CalculateGermanyTaxDetailed(input, settings)
}

func CalculateGermanyTaxDetailed(input GermanyTaxInput, settings config.GermanyTaxConfig) GermanyTaxBreakdown {
	breakdown := GermanyTaxBreakdown{
		GrossRealizedGain:            input.GrossRealizedGain,
		RealizedLoss:                 input.RealizedLoss,
		RealizedGain:                 input.RealizedGain,
		PartialExemptionAmount:       input.PartialExemptionAmount,
		TaxableAmountBeforeAllowance: input.TaxableAmountBeforeAllowance,
		WithholdingTaxPaid:           input.WithholdingTaxPaid,
	}

	if breakdown.TaxableAmountBeforeAllowance.LessThanOrEqual(decimal.Zero) {
		return breakdown
	}

	allowance := decimal.NewFromFloat(settings.AnnualAllowance)
	breakdown.AllowanceUsed = decimal.Min(breakdown.TaxableAmountBeforeAllowance, allowance)
	breakdown.TaxableAmount = decimal.Max(breakdown.TaxableAmountBeforeAllowance.Sub(breakdown.AllowanceUsed), decimal.Zero)
	breakdown.CapitalIncomeTax = breakdown.TaxableAmount.Mul(decimal.NewFromFloat(settings.CapitalIncomeTaxRate))
	breakdown.SolidaritySurcharge = breakdown.CapitalIncomeTax.Mul(decimal.NewFromFloat(settings.SolidaritySurchargeRate))
	breakdown.ChurchTax = breakdown.CapitalIncomeTax.Mul(decimal.NewFromFloat(settings.ChurchTaxRate))
	breakdown.TotalTax = breakdown.CapitalIncomeTax.Add(breakdown.SolidaritySurcharge).Add(breakdown.ChurchTax)
	breakdown.TaxCreditUsed = decimal.Min(breakdown.TotalTax, breakdown.WithholdingTaxPaid)
	breakdown.NetTaxDue = decimal.Max(breakdown.TotalTax.Sub(breakdown.TaxCreditUsed), decimal.Zero)

	return breakdown
}
