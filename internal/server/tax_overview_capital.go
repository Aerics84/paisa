package server

import (
	"fmt"

	"github.com/ananthakumaran/paisa/internal/config"
	c "github.com/ananthakumaran/paisa/internal/model/commodity"
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/ananthakumaran/paisa/internal/query"
	"github.com/ananthakumaran/paisa/internal/utils"
	"github.com/samber/lo"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func collectGermanyTaxPostings(db *gorm.DB) (investmentPostings []posting.Posting, incomePostings []posting.Posting, withholdingTaxPostings []posting.Posting) {
	commodities := lo.Filter(c.All(), func(commodity config.Commodity, _ int) bool {
		return commodity.Type == config.MutualFund || commodity.Type == config.Stock
	})
	investmentPostings = query.Init(db).Like("Assets:%").Commodities(commodities).All()
	incomePostings = query.Init(db).AccountPrefix("Income:Dividend", "Income:Interest").All()
	withholdingTaxPostings = query.Init(db).AccountPrefix("Expenses:Broker:Taxes").All()
	return investmentPostings, incomePostings, withholdingTaxPostings
}

func ComputeTaxOverviewCapitalIncome(investmentPostings, incomePostings, withholdingTaxPostings []posting.Posting, year string) (TaxOverviewCapitalIncome, []TaxOverviewDiagnostic) {
	result := TaxOverviewCapitalIncome{Accounts: []TaxOverviewCapitalIncomeAccount{}}
	diagnostics := []TaxOverviewDiagnostic{}

	byAccount := lo.GroupBy(investmentPostings, func(p posting.Posting) string { return p.Account })
	sellTransactionIDs := map[string]bool{}
	individualSales := 0
	unmatchedSells := []posting.Posting{}

	for _, account := range utils.SortedKeys(byAccount) {
		byYear, unmatchedByYear := computeGermanyTaxByYearWithUnmatched(account, byAccount[account])
		unmatchedSells = append(unmatchedSells, unmatchedByYear[year]...)
		capitalIncome, ok := byYear[year]
		if !ok {
			continue
		}

		for _, pair := range capitalIncome.PostingPairs {
			if pair.RealizedGain.GreaterThan(decimal.Zero) {
				result.RealizedGains = result.RealizedGains.Add(pair.RealizedGain)
			} else if pair.RealizedGain.LessThan(decimal.Zero) {
				result.RealizedLosses = result.RealizedLosses.Add(pair.RealizedGain.Neg())
			}

			if pair.Sell.TransactionID == "" {
				individualSales++
			} else {
				sellTransactionIDs[pair.Sell.TransactionID] = true
			}
		}

		result.Accounts = append(result.Accounts, TaxOverviewCapitalIncomeAccount{
			Account:      account,
			RealizedGain: capitalIncome.RealizedGain,
		})
	}
	result.Sales = len(sellTransactionIDs) + individualSales

	hasIncome := false
	for _, p := range incomePostings {
		if p.Date.Format("2006") != year {
			continue
		}
		if utils.IsSameOrParent(p.Account, "Income:Dividend") {
			result.Dividends = result.Dividends.Sub(p.Amount)
			hasIncome = true
		}
		if utils.IsSameOrParent(p.Account, "Income:Interest") {
			result.Interest = result.Interest.Sub(p.Amount)
			hasIncome = true
		}
	}

	hasWithholdingTaxPostings := false
	for _, p := range withholdingTaxPostings {
		if p.Date.Format("2006") != year {
			continue
		}
		if utils.IsSameOrParent(p.Account, "Expenses:Broker:Taxes") {
			hasWithholdingTaxPostings = true
			result.TaxesBooked = result.TaxesBooked.Add(p.Amount)
		}
	}

	if hasIncome && !hasWithholdingTaxPostings {
		diagnostics = append(diagnostics, TaxOverviewDiagnostic{
			Level:   "warning",
			Summary: "Dividendeneingänge ohne gebuchte Steuern; der Betrag kann netto sein. Bitte Broker-Abrechnung prüfen.",
			Details: "Für das ausgewählte Jahr gibt es Dividenden- oder Zinseingänge, aber keine Postings auf Expenses:Broker:Taxes. Paisa bruttoiert den Betrag nicht und klassifiziert keine Steuerart.",
		})
	}

	if len(unmatchedSells) > 0 {
		diagnostics = append(diagnostics, TaxOverviewDiagnostic{
			Level:   "warning",
			Summary: fmt.Sprintf("%d Verkaufspostings ohne passende Anschaffung; realisierte Gewinne/Verluste sind unvollständig.", len(unmatchedSells)),
			Details: "Für diese Verkäufe fehlen passende Kaufpostings im Ledger. Paisa bezieht nur gematchte Käufe in die realisierten Gewinne und Verluste ein: " + taxPostingExamples(unmatchedSells, 5),
		})
	}

	return result, diagnostics
}
