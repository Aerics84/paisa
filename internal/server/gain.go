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

func GetGain(db *gorm.DB) GainResponse {
	postings := query.Init(db).UntilToday().Like("Assets:%", "Income:CapitalGains:%").NotAccountPrefix("Assets:Checking").All()
	postings = service.PopulateMarketPrice(db, postings)
	byAccount := lo.GroupBy(postings, func(p posting.Posting) string {
		if service.IsCapitalGains(p) {
			return service.CapitalGainsSourceAccount(p.Account)
		}
		return p.Account
	})
	gains := make([]Gain, 0, len(byAccount))
	for _, account := range utils.SortedKeys(byAccount) {
		ps := byAccount[account]
		gains = append(gains, Gain{Account: account, XIRR: service.XIRR(db, ps), Networth: computeNetworth(db, ps), Postings: ps})
	}

	return GainResponse{GainBreakdown: gains}
}

func GetAccountGain(db *gorm.DB, account string) AccountGainResponse {
	capitalGainsAccount := strings.Replace(account, "Assets", "Income:CapitalGains", 1)
	postings := query.Init(db).UntilToday().AccountPrefix(account, capitalGainsAccount).All()
	postings = service.PopulateMarketPrice(db, postings)
	timeline := computeNetworthTimeline(db, postings, accounting.IsLeafAccount(db, account))
	if timeline == nil {
		timeline = []Networth{}
	}
	today := utils.Now()
	postingReturns := lo.FilterMap(postings, func(p posting.Posting, _ int) (service.PostingReturn, bool) {
		result, ok := service.ComputePostingReturn(p, today)
		if !ok {
			return service.PostingReturn{}, false
		}
		result.PostingID = p.ID
		return result, true
	})
	if postingReturns == nil {
		postingReturns = []service.PostingReturn{}
	}
	if postings == nil {
		postings = []posting.Posting{}
	}
	currencies := computeCurrencyAccountGains(db, postings, timeline)
	if currencies == nil {
		currencies = []CurrencyAccountGain{}
	}
	gain := AccountGain{Account: account, XIRR: service.XIRR(db, postings), NetworthTimeline: timeline, Postings: postings, PostingReturns: postingReturns, Currencies: currencies}

	commodities := lo.Uniq(lo.Map(postings, func(p posting.Posting, _ int) string { return p.Commodity }))
	var portfolio_groups PortfolioAllocationGroups
	portfolio_groups = GetAccountPortfolioAllocation(db, account)
	if !(len(commodities) > 0 && len(portfolio_groups.Commomdities) == len(commodities)) {
		portfolio_groups = PortfolioAllocationGroups{Commomdities: []string{}, NameAndSecurityType: []PortfolioAggregate{}, SecurityType: []PortfolioAggregate{}, Rating: []PortfolioAggregate{}, Industry: []PortfolioAggregate{}}
	}

	assetBreakdown := assets.ComputeBreakdown(db, postings, false, account)

	return AccountGainResponse{GainTimelineBreakdown: gain, PortfolioAllocation: portfolio_groups, AssetBreakdown: assetBreakdown}
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
