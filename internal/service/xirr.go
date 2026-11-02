package service

import (
	"time"

	"github.com/ananthakumaran/paisa/internal/model/cache"
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/ananthakumaran/paisa/internal/utils"
	"github.com/ananthakumaran/paisa/internal/xirr"
	"github.com/samber/lo"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func XIRR(db *gorm.DB, ps []posting.Posting) decimal.Decimal {
	return XIRRConverted(db, ps, func(time.Time) decimal.Decimal { return decimal.NewFromInt(1) })
}

// XIRRConverted computes XIRR after converting every cashflow into another
// currency using the rate (default currency per unit) of its date. A missing
// rate on the closing date makes the result unavailable instead of silently
// reporting a wrong return.
func XIRRConverted(db *gorm.DB, ps []posting.Posting, rate func(time.Time) decimal.Decimal) decimal.Decimal {
	today := utils.EndOfToday()
	todayRate := rate(today)
	if todayRate.IsZero() {
		return decimal.Zero
	}
	marketAmount := utils.SumBy(ps, func(p posting.Posting) decimal.Decimal {
		if IsCapitalGains(p) {
			return decimal.Zero
		}
		return p.MarketAmount
	})
	cashflows := lo.Reverse(lo.Map(ps, func(p posting.Posting, _ int) xirr.Cashflow {
		r := rate(p.Date)
		if IsInterest(db, p) || IsInterestRepayment(db, p) || r.IsZero() {
			return xirr.Cashflow{Date: p.Date, Amount: 0}
		} else {
			return xirr.Cashflow{Date: p.Date, Amount: p.Amount.Neg().Div(r).Round(4).InexactFloat64()}
		}
	}))

	cashflows = append(cashflows, xirr.Cashflow{Date: today, Amount: marketAmount.Div(todayRate).Round(4).InexactFloat64()})
	return cache.Lookup(db, cashflows, func() decimal.Decimal {
		return xirr.XIRR(cashflows)
	})
}

func APR(db *gorm.DB, ps []posting.Posting) decimal.Decimal {
	today := utils.EndOfToday()
	marketAmount := utils.SumBy(ps, func(p posting.Posting) decimal.Decimal {
		return p.MarketAmount
	})
	cashflows := lo.Map(ps, func(p posting.Posting, _ int) xirr.Cashflow {
		return xirr.Cashflow{Date: p.Date, Amount: p.Amount.Round(4).InexactFloat64()}
	})
	cashflows = append(cashflows, xirr.Cashflow{Date: today, Amount: marketAmount.Neg().Round(4).InexactFloat64()})

	return cache.Lookup(db, cashflows, func() decimal.Decimal {
		return xirr.XIRR(cashflows)
	})
}
