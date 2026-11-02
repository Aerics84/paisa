package taxation

import (
	"time"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/model/cii"
	"github.com/ananthakumaran/paisa/internal/service"
	"github.com/ananthakumaran/paisa/internal/utils"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// Tax boundaries are resolved lazily because config.LoadConfig may run after
// package initialization and can change the configured time zone.
func configuredDate(value string) time.Time {
	parsed, _ := time.ParseInLocation("2006-01-02", value, config.TimeZone())
	return parsed
}

func equityGrandfatherDate() time.Time        { return configuredDate("2018-02-01") }
func debtIndexationRevocationDate() time.Time { return configuredDate("2023-04-01") }
func ciiStartDate() time.Time                 { return configuredDate("2001-03-31") }
func equityLongTermTaxStartDate() time.Time   { return configuredDate("2018-04-01") }
func equityRateChangeDate() time.Time         { return configuredDate("2024-07-23") }

type Tax struct {
	Gain      decimal.Decimal `json:"gain"`
	Taxable   decimal.Decimal `json:"taxable"`
	Slab      decimal.Decimal `json:"slab"`
	LongTerm  decimal.Decimal `json:"long_term"`
	ShortTerm decimal.Decimal `json:"short_term"`
}

func Add(a, b Tax) Tax {
	return Tax{Gain: a.Gain.Add(b.Gain), Taxable: a.Taxable.Add(b.Taxable), LongTerm: a.LongTerm.Add(b.LongTerm), ShortTerm: a.ShortTerm.Add(b.ShortTerm), Slab: a.Slab.Add(b.Slab)}
}

func Calculate(db *gorm.DB, quantity decimal.Decimal, commodity config.Commodity, purchasePrice decimal.Decimal, purchaseDate time.Time, sellPrice decimal.Decimal, sellDate time.Time) Tax {

	isEquity := commodity.TaxCategory == config.Equity || commodity.TaxCategory == config.Equity65
	heldOverAYear := calendarYearsExceed(purchaseDate, sellDate, 1)
	gain := sellPrice.Mul(quantity).Sub(purchasePrice.Mul(quantity))

	if isEquity && heldOverAYear && sellDate.Before(equityLongTermTaxStartDate()) {
		return Tax{Gain: gain, Taxable: decimal.Zero, ShortTerm: decimal.Zero, LongTerm: decimal.Zero, Slab: decimal.Zero}
	}

	if isEquity && heldOverAYear && purchaseDate.Before(equityGrandfatherDate()) {
		fmv := service.GetUnitPrice(db, commodity.Name, equityGrandfatherDate().AddDate(0, 0, -1)).Value
		if fmv.IsPositive() {
			purchasePrice = grandfatheredPurchasePrice(purchasePrice, fmv, sellPrice)
		}
	}

	if commodity.TaxCategory == config.Debt && purchaseDate.After(ciiStartDate()) && purchaseDate.Before(debtIndexationRevocationDate()) && calendarYearsExceed(purchaseDate, sellDate, 3) {
		purchasePrice = purchasePrice.Mul(decimal.NewFromInt(int64(cii.GetIndex(db, utils.FY(sellDate)))).Div(decimal.NewFromInt(int64(cii.GetIndex(db, utils.FY(purchaseDate))))))
	}

	if commodity.TaxCategory == config.UnlistedEquity && purchaseDate.After(ciiStartDate()) && calendarYearsExceed(purchaseDate, sellDate, 2) {
		purchasePrice = purchasePrice.Mul(decimal.NewFromInt(int64(cii.GetIndex(db, utils.FY(sellDate)))).Div(decimal.NewFromInt(int64(cii.GetIndex(db, utils.FY(purchaseDate))))))
	}

	taxable := sellPrice.Mul(quantity).Sub(purchasePrice.Mul(quantity))
	shortTerm := decimal.Zero
	longTerm := decimal.Zero
	slab := decimal.Zero

	if isEquity {
		shortTermRate, longTermRate := equityRates(sellDate)
		if heldOverAYear {
			longTerm = taxable.Mul(longTermRate)
		} else {
			shortTerm = taxable.Mul(shortTermRate)
		}
	}

	if commodity.TaxCategory == config.Debt {
		if calendarYearsExceed(purchaseDate, sellDate, 3) && purchaseDate.Before(debtIndexationRevocationDate()) {
			longTerm = taxable.Mul(decimal.NewFromFloat(0.20))
		} else {
			slab = taxable
		}
	}

	if commodity.TaxCategory == config.Equity35 {
		if calendarYearsExceed(purchaseDate, sellDate, 3) {
			longTerm = taxable.Mul(decimal.NewFromFloat(0.20))
		} else {
			slab = taxable
		}
	}

	if commodity.TaxCategory == config.UnlistedEquity {
		if calendarYearsExceed(purchaseDate, sellDate, 2) {
			longTerm = taxable.Mul(decimal.NewFromFloat(0.20))
		} else {
			slab = taxable
		}
	}

	return Tax{Gain: gain, Taxable: taxable, ShortTerm: shortTerm, LongTerm: longTerm, Slab: slab}
}

func equityRates(sellDate time.Time) (shortTerm, longTerm decimal.Decimal) {
	if sellDate.Before(equityRateChangeDate()) {
		return decimal.NewFromFloat(0.15), decimal.NewFromFloat(0.10)
	}

	return decimal.NewFromFloat(0.20), decimal.NewFromFloat(0.125)
}

func grandfatheredPurchasePrice(actual, fmv, sale decimal.Decimal) decimal.Decimal {
	return decimal.Max(actual, decimal.Min(fmv, sale))
}

func calendarYearsExceed(start, end time.Time, years int) bool {
	return end.After(start.AddDate(years, 0, 0))
}
