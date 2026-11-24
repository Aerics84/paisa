package taxation

import (
	"github.com/shopspring/decimal"
)

// Wire types shared by the HTTP API. They live in this file so the tygo
// generation can include only the response DTOs instead of the whole
// calculation package.

type Tax struct {
	Gain      decimal.Decimal `json:"gain"`
	Taxable   decimal.Decimal `json:"taxable"`
	Slab      decimal.Decimal `json:"slab"`
	LongTerm  decimal.Decimal `json:"long_term"`
	ShortTerm decimal.Decimal `json:"short_term"`
}

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
