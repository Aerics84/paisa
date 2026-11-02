package server

import (
	"strings"

	"github.com/ananthakumaran/paisa/internal/accounting"
	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/ananthakumaran/paisa/internal/query"
	"github.com/ananthakumaran/paisa/internal/server/assets"
	"github.com/ananthakumaran/paisa/internal/service"
	"github.com/ananthakumaran/paisa/internal/utils"
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type Gain struct {
	Account  string            `json:"account"`
	Networth Networth          `json:"networth"`
	XIRR     decimal.Decimal   `json:"xirr"`
	Postings []posting.Posting `json:"postings"`
}

type CurrencyAccountGain struct {
	Name             string            `json:"name"`
	Symbol           string            `json:"symbol"`
	Commodity        string            `json:"commodity"`
	NetworthTimeline []Networth        `json:"networthTimeline"`
	XIRR             decimal.Decimal   `json:"xirr"`
	Postings         []posting.Posting `json:"postings"`
}

type AccountGain struct {
	Account          string                  `json:"account"`
	NetworthTimeline []Networth              `json:"networthTimeline"`
	XIRR             decimal.Decimal         `json:"xirr"`
	Postings         []posting.Posting       `json:"postings"`
	PostingReturns   []service.PostingReturn `json:"posting_returns"`
	Currencies       []CurrencyAccountGain   `json:"currencies"`
}

func GetGain(db *gorm.DB) gin.H {
	postings := query.Init(db).UntilToday().Like("Assets:%", "Income:CapitalGains:%").NotAccountPrefix("Assets:Checking").All()
	postings = service.PopulateMarketPrice(db, postings)
	byAccount := lo.GroupBy(postings, func(p posting.Posting) string {
		if service.IsCapitalGains(p) {
			return service.CapitalGainsSourceAccount(p.Account)
		}
		return p.Account
	})
	var gains []Gain
	for _, account := range utils.SortedKeys(byAccount) {
		ps := byAccount[account]
		gains = append(gains, Gain{Account: account, XIRR: service.XIRR(db, ps), Networth: computeNetworth(db, ps), Postings: ps})
	}

	return gin.H{"gain_breakdown": gains}
}

func GetAccountGain(db *gorm.DB, account string) gin.H {
	capitalGainsAccount := strings.Replace(account, "Assets", "Income:CapitalGains", 1)
	postings := query.Init(db).UntilToday().AccountPrefix(account, capitalGainsAccount).All()
	postings = service.PopulateMarketPrice(db, postings)
	timeline := computeNetworthTimeline(db, postings, accounting.IsLeafAccount(db, account))
	today := utils.Now()
	postingReturns := lo.FilterMap(postings, func(p posting.Posting, _ int) (service.PostingReturn, bool) {
		return service.ComputePostingReturn(p, today)
	})
	gain := AccountGain{Account: account, XIRR: service.XIRR(db, postings), NetworthTimeline: timeline, Postings: postings, PostingReturns: postingReturns, Currencies: computeCurrencyAccountGains(db, postings, timeline)}

	commodities := lo.Uniq(lo.Map(postings, func(p posting.Posting, _ int) string { return p.Commodity }))
	var portfolio_groups PortfolioAllocationGroups
	portfolio_groups = GetAccountPortfolioAllocation(db, account)
	if !(len(commodities) > 0 && len(portfolio_groups.Commomdities) == len(commodities)) {
		portfolio_groups = PortfolioAllocationGroups{Commomdities: []string{}, NameAndSecurityType: []PortfolioAggregate{}, SecurityType: []PortfolioAggregate{}, Rating: []PortfolioAggregate{}, Industry: []PortfolioAggregate{}}
	}

	assetBreakdown := assets.ComputeBreakdown(db, postings, false, account)

	return gin.H{"gain_timeline_breakdown": gain, "portfolio_allocation": portfolio_groups, "asset_breakdown": assetBreakdown}
}

func computeCurrencyAccountGains(db *gorm.DB, postings []posting.Posting, timeline []Networth) []CurrencyAccountGain {
	currencies := config.GetConfig().NetworthCurrencies
	result := make([]CurrencyAccountGain, 0, len(currencies))
	for _, nc := range currencies {
		symbol := nc.Symbol
		if symbol == "" {
			symbol = nc.Commodity
		}

		gain := CurrencyAccountGain{
			Name:             nc.Name,
			Symbol:           symbol,
			Commodity:        nc.Commodity,
			NetworthTimeline: computeConvertedNetworthTimeline(db, nc.Commodity, timeline),
			Postings:         []posting.Posting{},
		}

		if rateAt := currencyRateFn(db, nc.Commodity); rateAt != nil {
			gain.XIRR = service.XIRRConverted(db, postings, rateAt)
			gain.Postings = lo.FilterMap(postings, func(p posting.Posting, _ int) (posting.Posting, bool) {
				rate := rateAt(p.Date)
				if rate.IsZero() {
					return p, false
				}
				p.Amount = p.Amount.Div(rate)
				p.MarketAmount = p.MarketAmount.Div(rate)
				return p, true
			})
		}
		result = append(result, gain)
	}
	return result
}
