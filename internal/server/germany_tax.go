package server

import (
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
	Purchase     posting.Posting `json:"purchase"`
	Sell         posting.Posting `json:"sell"`
	RealizedGain decimal.Decimal `json:"realized_gain"`
}

type GermanyTaxAccount struct {
	Account       string                  `json:"account"`
	Units         decimal.Decimal         `json:"units"`
	PurchasePrice decimal.Decimal         `json:"purchase_price"`
	SellPrice     decimal.Decimal         `json:"sell_price"`
	RealizedGain  decimal.Decimal         `json:"realized_gain"`
	PostingPairs  []GermanyTaxPostingPair `json:"posting_pairs"`
}

type GermanyTaxYear struct {
	TaxYear  string                       `json:"tax_year"`
	Settings config.GermanyTaxConfig      `json:"settings"`
	Summary  taxation.GermanyTaxBreakdown `json:"summary"`
	Accounts []GermanyTaxAccount          `json:"accounts"`
}

func GetGermanyTax(db *gorm.DB) gin.H {
	if !config.SupportsGermanyTaxFeatures() {
		return gin.H{"tax_years": map[string]GermanyTaxYear{}}
	}

	commodities := lo.Filter(c.All(), func(commodity config.Commodity, _ int) bool {
		return commodity.Type == config.MutualFund || commodity.Type == config.Stock
	})
	postings := query.Init(db).Like("Assets:%").Commodities(commodities).All()
	byAccount := lo.GroupBy(postings, func(p posting.Posting) string { return p.Account })
	years := map[string]*GermanyTaxYear{}

	for _, account := range utils.SortedKeys(byAccount) {
		for year, capitalIncome := range computeGermanyTaxByYear(account, byAccount[account]) {
			reportYear := years[year]
			if reportYear == nil {
				reportYear = &GermanyTaxYear{
					TaxYear:  year,
					Settings: config.GetConfig().GermanyTax,
					Summary:  taxation.GermanyTaxBreakdown{},
					Accounts: []GermanyTaxAccount{},
				}
				years[year] = reportYear
			}
			reportYear.Accounts = append(reportYear.Accounts, capitalIncome)
		}
	}

	result := lo.MapValues(years, func(reportYear *GermanyTaxYear, _ string) GermanyTaxYear {
		realizedGain := utils.SumBy(reportYear.Accounts, func(account GermanyTaxAccount) decimal.Decimal {
			return account.RealizedGain
		})
		reportYear.Summary = taxation.CalculateGermanyTax(realizedGain, reportYear.Settings)
		return *reportYear
	})

	return gin.H{"tax_years": result}
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
			taxYear := currentPosting.Date.Format("2006")
			yearCapitalIncome := byYear[taxYear]
			yearCapitalIncome.Account = account
			yearCapitalIncome.Units = yearCapitalIncome.Units.Add(matchedQuantity)
			yearCapitalIncome.PurchasePrice = yearCapitalIncome.PurchasePrice.Add(purchasePrice)
			yearCapitalIncome.SellPrice = yearCapitalIncome.SellPrice.Add(sellPrice)
			yearCapitalIncome.RealizedGain = yearCapitalIncome.RealizedGain.Add(realizedGain)
			yearCapitalIncome.PostingPairs = append(yearCapitalIncome.PostingPairs, GermanyTaxPostingPair{
				Purchase:     first.WithQuantity(matchedQuantity),
				Sell:         currentPosting.WithQuantity(matchedQuantity.Neg()),
				RealizedGain: realizedGain,
			})
			byYear[taxYear] = yearCapitalIncome
		}
	}

	return byYear
}
