package server

import (
	"fmt"

	"github.com/ananthakumaran/paisa/internal/config"
	c "github.com/ananthakumaran/paisa/internal/model/commodity"
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/ananthakumaran/paisa/internal/query"
	"github.com/ananthakumaran/paisa/internal/taxation"
	"github.com/ananthakumaran/paisa/internal/utils"
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type GermanyTaxPostingPair struct {
	Purchase                posting.Posting `json:"purchase"`
	Sell                    posting.Posting `json:"sell"`
	RealizedGain            decimal.Decimal `json:"realized_gain"`
	PartialExemptionRate    *float64        `json:"partial_exemption_rate"`
	PartialExemptionAmount  decimal.Decimal `json:"partial_exemption_amount"`
	TaxableGain             decimal.Decimal `json:"taxable_gain"`
}

type GermanyTaxAccount struct {
	Account                string                  `json:"account"`
	Commodity              string                  `json:"commodity"`
	CommodityType          config.CommodityType    `json:"commodity_type"`
	PartialExemptionRate   *float64                `json:"partial_exemption_rate"`
	Units                  decimal.Decimal         `json:"units"`
	PurchasePrice          decimal.Decimal         `json:"purchase_price"`
	SellPrice              decimal.Decimal         `json:"sell_price"`
	RealizedGain           decimal.Decimal         `json:"realized_gain"`
	PartialExemptionAmount decimal.Decimal         `json:"partial_exemption_amount"`
	TaxableGain            decimal.Decimal         `json:"taxable_gain"`
	PostingPairs           []GermanyTaxPostingPair `json:"posting_pairs"`
}

type GermanyTaxDiagnostic struct {
	Level   string `json:"level"`
	Summary string `json:"summary"`
	Details string `json:"details"`
}

type GermanyTaxYear struct {
	TaxYear     string                       `json:"tax_year"`
	Settings    config.GermanyTaxConfig      `json:"settings"`
	Summary     taxation.GermanyTaxBreakdown `json:"summary"`
	Accounts    []GermanyTaxAccount          `json:"accounts"`
	Diagnostics []GermanyTaxDiagnostic       `json:"diagnostics"`
}

func GetGermanyTax(db *gorm.DB) gin.H {
	if !config.SupportsGermanyTaxFeatures() {
		return gin.H{"tax_years": map[string]GermanyTaxYear{}}
	}

	commodities := lo.Filter(c.All(), func(commodity config.Commodity, _ int) bool {
		return commodity.Type == config.MutualFund || commodity.Type == config.Stock
	})
	postings := query.Init(db).Like("Assets:%").Commodities(commodities).All()
	incomePostings := query.Init(db).AccountPrefix("Income:Dividend", "Income:Interest").All()
	withholdingTaxPostings := query.Init(db).AccountPrefix("Expenses:Broker:Taxes").All()
	byAccount := lo.GroupBy(postings, func(p posting.Posting) string { return p.Account })
	years := map[string]*GermanyTaxYear{}

	for _, account := range utils.SortedKeys(byAccount) {
		for year, capitalIncome := range computeGermanyTaxByYear(account, byAccount[account]) {
			reportYear := years[year]
			if reportYear == nil {
				reportYear = &GermanyTaxYear{
					TaxYear:     year,
					Settings:    config.GetConfig().GermanyTax,
					Summary:     taxation.GermanyTaxBreakdown{},
					Accounts:    []GermanyTaxAccount{},
					Diagnostics: []GermanyTaxDiagnostic{},
				}
				years[year] = reportYear
			}
			reportYear.Accounts = append(reportYear.Accounts, capitalIncome)
		}
	}

	incomePostingsByYear := lo.GroupBy(incomePostings, func(p posting.Posting) string { return p.Date.Format("2006") })
	withholdingTaxPostingsByYear := lo.GroupBy(withholdingTaxPostings, func(p posting.Posting) string { return p.Date.Format("2006") })

	result := lo.MapValues(years, func(reportYear *GermanyTaxYear, _ string) GermanyTaxYear {
		finalized := buildGermanyTaxYear(
			reportYear.TaxYear,
			reportYear.Settings,
			reportYear.Accounts,
			incomePostingsByYear[reportYear.TaxYear],
			withholdingTaxPostingsByYear[reportYear.TaxYear],
		)
		reportYear.Summary = finalized.Summary
		reportYear.Diagnostics = finalized.Diagnostics
		return *reportYear
	})

	return gin.H{"tax_years": result}
}

func buildGermanyTaxYear(
	year string,
	settings config.GermanyTaxConfig,
	accounts []GermanyTaxAccount,
	incomePostings []posting.Posting,
	withholdingTaxPostings []posting.Posting,
) GermanyTaxYear {
	reportYear := GermanyTaxYear{
		TaxYear:     year,
		Settings:    settings,
		Accounts:    accounts,
		Diagnostics: []GermanyTaxDiagnostic{},
	}

	input := taxation.GermanyTaxInput{}
	for _, account := range accounts {
		if account.RealizedGain.GreaterThan(decimal.Zero) {
			input.GrossRealizedGain = input.GrossRealizedGain.Add(account.RealizedGain)
		} else if account.RealizedGain.LessThan(decimal.Zero) {
			input.RealizedLoss = input.RealizedLoss.Add(account.RealizedGain.Neg())
		}
		input.RealizedGain = input.RealizedGain.Add(account.RealizedGain)
		input.PartialExemptionAmount = input.PartialExemptionAmount.Add(account.PartialExemptionAmount)
		input.TaxableAmountBeforeAllowance = input.TaxableAmountBeforeAllowance.Add(account.TaxableGain)

		if account.CommodityType == config.MutualFund && account.PartialExemptionRate == nil {
			reportYear.Diagnostics = append(reportYear.Diagnostics, GermanyTaxDiagnostic{
				Level:   "warning",
				Summary: fmt.Sprintf("Missing ETF partial-exemption metadata for %s", account.Commodity),
				Details: fmt.Sprintf("Account %s has Germany tax activity for commodity %s, but no germany_partial_exemption_rate is configured. Paisa treated the position as fully taxable for this year.", account.Account, account.Commodity),
			})
		}
	}

	input.WithholdingTaxPaid = utils.SumBy(withholdingTaxPostings, func(p posting.Posting) decimal.Decimal {
		return p.Amount
	})
	reportYear.Summary = taxation.CalculateGermanyTaxDetailed(input, settings)

	if len(incomePostings) > 0 && reportYear.Summary.WithholdingTaxPaid.IsZero() {
		reportYear.Diagnostics = append(reportYear.Diagnostics, GermanyTaxDiagnostic{
			Level:   "warning",
			Summary: "Dividend or interest income has no withholding-tax inputs",
			Details: "This tax year includes dividend or interest postings, but no Expenses:Broker:Taxes postings were found. Net Germany tax due may be overstated until withholding taxes are captured in the ledger.",
		})
	}

	return reportYear
}

func computeGermanyTaxByYear(account string, postings []posting.Posting) map[string]GermanyTaxAccount {
	byYear := make(map[string]GermanyTaxAccount)
	var available []posting.Posting

	for _, currentPosting := range postings {
		if currentPosting.Quantity.GreaterThan(decimal.Zero) {
			available = append(available, currentPosting)
			continue
		}

		remaining := currentPosting.Quantity.Neg()
		for remaining.GreaterThan(decimal.Zero) && len(available) > 0 {
			first := available[0]
			matchedQuantity := decimal.Zero

			if first.Quantity.GreaterThan(remaining) {
				first.AddQuantity(remaining.Neg())
				matchedQuantity = remaining
				available[0] = first
				remaining = decimal.Zero
			} else {
				remaining = remaining.Sub(first.Quantity)
				matchedQuantity = first.Quantity
				available = available[1:]
			}

			purchasePrice := matchedQuantity.Mul(first.Price())
			sellPrice := matchedQuantity.Mul(currentPosting.Price())
			realizedGain := sellPrice.Sub(purchasePrice)
			commodity := c.FindByName(currentPosting.Commodity)
			partialExemptionAmount := decimal.Zero
			taxableGain := realizedGain
			if commodity.GermanyPartialExemptionRate != nil {
				multiplier := decimal.NewFromInt(1).Sub(decimal.NewFromFloat(*commodity.GermanyPartialExemptionRate))
				taxableGain = realizedGain.Mul(multiplier)
				partialExemptionAmount = realizedGain.Sub(taxableGain)
			}
			taxYear := currentPosting.Date.Format("2006")
			yearCapitalIncome := byYear[taxYear]
			yearCapitalIncome.Account = account
			yearCapitalIncome.Commodity = currentPosting.Commodity
			yearCapitalIncome.CommodityType = commodity.Type
			yearCapitalIncome.PartialExemptionRate = commodity.GermanyPartialExemptionRate
			yearCapitalIncome.Units = yearCapitalIncome.Units.Add(matchedQuantity)
			yearCapitalIncome.PurchasePrice = yearCapitalIncome.PurchasePrice.Add(purchasePrice)
			yearCapitalIncome.SellPrice = yearCapitalIncome.SellPrice.Add(sellPrice)
			yearCapitalIncome.RealizedGain = yearCapitalIncome.RealizedGain.Add(realizedGain)
			yearCapitalIncome.PartialExemptionAmount = yearCapitalIncome.PartialExemptionAmount.Add(partialExemptionAmount)
			yearCapitalIncome.TaxableGain = yearCapitalIncome.TaxableGain.Add(taxableGain)
			yearCapitalIncome.PostingPairs = append(yearCapitalIncome.PostingPairs, GermanyTaxPostingPair{
				Purchase:               first.WithQuantity(matchedQuantity),
				Sell:                   currentPosting.WithQuantity(matchedQuantity.Neg()),
				RealizedGain:           realizedGain,
				PartialExemptionRate:   commodity.GermanyPartialExemptionRate,
				PartialExemptionAmount: partialExemptionAmount,
				TaxableGain:            taxableGain,
			})
			byYear[taxYear] = yearCapitalIncome
		}
	}

	return byYear
}
