package server

import (
	"time"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/ananthakumaran/paisa/internal/query"
	"github.com/ananthakumaran/paisa/internal/service"
	"github.com/ananthakumaran/paisa/internal/utils"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type Networth struct {
	Date                time.Time       `json:"date"`
	InvestmentAmount    decimal.Decimal `json:"investmentAmount"`
	WithdrawalAmount    decimal.Decimal `json:"withdrawalAmount"`
	GainAmount          decimal.Decimal `json:"gainAmount"`
	BalanceAmount       decimal.Decimal `json:"balanceAmount"`
	BalanceUnits        decimal.Decimal `json:"balanceUnits"`
	NetInvestmentAmount decimal.Decimal `json:"netInvestmentAmount"`
}

func GetNetworth(db *gorm.DB) gin.H {
	postings := query.Init(db).Like("Assets:%", "Income:CapitalGains:%", "Liabilities:%").UntilToday().All()

	postings = service.PopulateMarketPrice(db, postings)
	networthTimeline := computeNetworthTimeline(db, postings, false)
	currencyNetworths := computeCurrencyNetworths(db, networthTimeline)
	xirr := service.XIRR(db, postings)
	return gin.H{"networthTimeline": networthTimeline, "currencyNetworths": currencyNetworths, "xirr": xirr}
}

func GetCurrentNetworth(db *gorm.DB) gin.H {
	postings := query.Init(db).Like("Assets:%", "Income:CapitalGains:%", "Liabilities:%").UntilToday().All()
	postings = service.PopulateMarketPrice(db, postings)
	networth := computeNetworth(db, postings)
	xirr := service.XIRR(db, postings)
	return gin.H{"networth": networth, "xirr": xirr}
}

func computeNetworth(db *gorm.DB, postings []posting.Posting) Networth {
	var networth Networth

	if len(postings) == 0 {
		return networth
	}

	var investment decimal.Decimal = decimal.Zero
	var withdrawal decimal.Decimal = decimal.Zero
	var balance decimal.Decimal = decimal.Zero

	now := utils.EndOfToday()
	for _, p := range postings {
		isInterest := service.IsInterest(db, p)
		isInterestRepayment := service.IsInterestRepayment(db, p)
		isStockSplit := service.IsStockSplit(db, p)
		isCapitalGains := service.IsCapitalGains(p)

		if isInterest || isInterestRepayment {
			balance = balance.Add(p.Amount)
		} else if isCapitalGains {
			withdrawal = withdrawal.Add(p.Amount.Neg())
		} else {
			if p.Amount.GreaterThan(decimal.Zero) && !isStockSplit {
				investment = investment.Add(p.Amount)
			}

			if p.Amount.LessThan(decimal.Zero) && !isStockSplit {
				withdrawal = withdrawal.Add(p.Amount.Neg())
			}

			balance = balance.Add(service.GetMarketPrice(db, p, now))
		}
	}

	gain := balance.Add(withdrawal).Sub(investment)
	netInvestment := investment.Sub(withdrawal)
	networth = Networth{
		Date:                now,
		InvestmentAmount:    investment,
		WithdrawalAmount:    withdrawal,
		GainAmount:          gain,
		BalanceAmount:       balance,
		NetInvestmentAmount: netInvestment,
	}

	return networth
}

func computeNetworthTimeline(db *gorm.DB, postings []posting.Posting, computeBalanceUnits bool) []Networth {
	var networths []Networth

	var p posting.Posting

	if len(postings) == 0 {
		return []Networth{}
	}

	type RunningSum struct {
		investment   decimal.Decimal
		withdrawal   decimal.Decimal
		balance      decimal.Decimal
		balanceUnits decimal.Decimal
	}

	accumulator := make(map[string]RunningSum)

	end := utils.EndOfToday()
	for start := postings[0].Date; start.Before(end); start = start.AddDate(0, 0, 1) {
		for len(postings) > 0 && (postings[0].Date.Before(start) || postings[0].Date.Equal(start)) {
			p, postings = postings[0], postings[1:]
			rs := accumulator[p.Commodity]

			isInterest := service.IsInterest(db, p)
			isCapitalGains := service.IsCapitalGains(p)

			if p.Amount.GreaterThan(decimal.Zero) && !isInterest {
				rs.investment = rs.investment.Add(p.Amount)
			}

			if p.Amount.LessThan(decimal.Zero) && !isInterest {
				rs.withdrawal = rs.withdrawal.Add(p.Amount.Neg())
			}

			if !isCapitalGains {
				rs.balance = rs.balance.Add(service.GetMarketPrice(db, p, start))
				rs.balanceUnits = rs.balanceUnits.Add(p.Quantity)
			}

			accumulator[p.Commodity] = rs

		}

		var investment decimal.Decimal = decimal.Zero
		var withdrawal decimal.Decimal = decimal.Zero
		var balance decimal.Decimal = decimal.Zero
		var balanceUnits decimal.Decimal = decimal.Zero

		for commodity, rs := range accumulator {
			investment = investment.Add(rs.investment)
			withdrawal = withdrawal.Add(rs.withdrawal)

			if utils.IsCurrency(commodity) {
				balance = balance.Add(rs.balance)
			} else {
				if computeBalanceUnits {
					balanceUnits = balanceUnits.Add(rs.balanceUnits)
				}
				price := service.GetUnitPrice(db, commodity, start)
				if !price.Value.Equal(decimal.Zero) {
					balance = balance.Add(rs.balanceUnits.Mul(price.Value))
				} else {
					balance = balance.Add(rs.balance)
				}
			}

		}

		gain := balance.Add(withdrawal).Sub(investment)
		netInvestment := investment.Sub(withdrawal)
		networths = append(networths, Networth{
			Date:                start,
			InvestmentAmount:    investment,
			WithdrawalAmount:    withdrawal,
			GainAmount:          gain,
			BalanceAmount:       balance,
			BalanceUnits:        balanceUnits,
			NetInvestmentAmount: netInvestment,
		})

		if len(postings) == 0 && balance.Abs().LessThan(decimal.NewFromFloat(0.01)) {
			break
		}
	}
	return networths
}

type CurrencyNetworth struct {
	Name             string     `json:"name"`
	Symbol           string     `json:"symbol"`
	Commodity        string     `json:"commodity"`
	NetworthTimeline []Networth `json:"networthTimeline"`
}

func computeCurrencyNetworths(db *gorm.DB, timeline []Networth) []CurrencyNetworth {
	currencies := config.GetConfig().NetworthCurrencies
	result := make([]CurrencyNetworth, 0, len(currencies))
	for _, nc := range currencies {
		symbol := nc.Symbol
		if symbol == "" {
			symbol = nc.Commodity
		}
		result = append(result, CurrencyNetworth{
			Name:             nc.Name,
			Symbol:           symbol,
			Commodity:        nc.Commodity,
			NetworthTimeline: computeConvertedNetworthTimeline(db, nc.Commodity, timeline),
		})
	}
	return result
}

// computeConvertedNetworthTimeline restates the timeline in the currency whose
// rate is the price of the commodity. Balances are converted at the rate of the
// day, while each day's net new investment / withdrawal is converted at that
// day's rate, so the gain reflects both asset performance and currency
// movement.
func computeConvertedNetworthTimeline(db *gorm.DB, commodity string, timeline []Networth) []Networth {
	converted := []Networth{}
	if len(timeline) == 0 || utils.IsCurrency(commodity) {
		return converted
	}

	rateAt := currencyRateFn(db, commodity)
	if rateAt == nil {
		return converted
	}

	investment := decimal.Zero
	withdrawal := decimal.Zero
	prev := Networth{}
	for _, n := range timeline {
		rate := rateAt(n.Date)
		if rate.IsZero() {
			continue
		}

		investment = investment.Add(n.InvestmentAmount.Sub(prev.InvestmentAmount).Div(rate))
		withdrawal = withdrawal.Add(n.WithdrawalAmount.Sub(prev.WithdrawalAmount).Div(rate))
		balance := n.BalanceAmount.Div(rate)
		prev = n

		converted = append(converted, Networth{
			Date:                n.Date,
			InvestmentAmount:    investment,
			WithdrawalAmount:    withdrawal,
			GainAmount:          balance.Add(withdrawal).Sub(investment),
			BalanceAmount:       balance,
			NetInvestmentAmount: investment.Sub(withdrawal),
		})
	}
	return converted
}

// currencyRateFn returns a lookup of the rate (default currency per unit of the
// commodity) on a date, or nil when no price is available. Dates before the
// first known rate use the earliest rate.
func currencyRateFn(db *gorm.DB, commodity string) func(time.Time) decimal.Decimal {
	if commodity == "" || utils.IsCurrency(commodity) {
		return nil
	}

	rates := service.CurrencyRates(db, commodity)
	if len(rates) == 0 {
		return nil
	}

	return func(date time.Time) decimal.Decimal {
		rate := rates[0].Value
		for _, r := range rates {
			if r.Date.After(date) {
				break
			}
			rate = r.Value
		}
		return rate
	}
}
