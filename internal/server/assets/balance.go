package assets

import (
	"sort"
	"strings"
	"time"

	"github.com/samber/lo"
	"github.com/shopspring/decimal"

	"github.com/ananthakumaran/paisa/internal/accounting"
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/ananthakumaran/paisa/internal/query"
	"github.com/ananthakumaran/paisa/internal/service"
	"github.com/ananthakumaran/paisa/internal/utils"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AssetBreakdown struct {
	Group            string          `json:"group"`
	InvestmentAmount decimal.Decimal `json:"investmentAmount"`
	WithdrawalAmount decimal.Decimal `json:"withdrawalAmount"`
	MarketAmount     decimal.Decimal `json:"marketAmount"`
	BalanceUnits     decimal.Decimal `json:"balanceUnits"`
	LatestPrice      decimal.Decimal `json:"latestPrice"`
	XIRR             decimal.Decimal `json:"xirr"`
	GainAmount       decimal.Decimal `json:"gainAmount"`
	AbsoluteReturn   decimal.Decimal `json:"absoluteReturn"`
}

// CheckingBalancePoint is the recorded end-of-month balance of one checking
// account. It never includes market prices; checking accounts are cash.
type CheckingBalancePoint struct {
	Month   time.Time       `json:"month"`
	Account string          `json:"account"`
	Balance decimal.Decimal `json:"balance"`
}

// AvailableMonthRange is the picker bound derived from recorded postings. It
// is not a claim that the ledger history is complete.
type AvailableMonthRange struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

// GetCheckingBalanceTimeline returns the end-of-month balance of every
// checking account for the latest twelve recorded calendar months.
func GetCheckingBalanceTimeline(db *gorm.DB) []CheckingBalancePoint {
	points := make([]CheckingBalancePoint, 0)
	postings := query.Init(db).AccountPrefix("Assets:Checking").All()
	if len(postings) == 0 {
		return points
	}

	monthByKey := make(map[string]time.Time)
	flowsByAccount := make(map[string]map[string]decimal.Decimal)
	firstMonthByAccount := make(map[string]time.Time)

	for _, p := range postings {
		month := utils.BeginningOfMonth(p.Date)
		key := month.Format("2006-01")
		monthByKey[key] = month

		if flowsByAccount[p.Account] == nil {
			flowsByAccount[p.Account] = make(map[string]decimal.Decimal)
		}
		flowsByAccount[p.Account][key] = flowsByAccount[p.Account][key].Add(p.Amount)

		if first, found := firstMonthByAccount[p.Account]; !found || month.Before(first) {
			firstMonthByAccount[p.Account] = month
		}
	}

	keys := utils.SortedKeys(monthByKey)
	if len(keys) > 12 {
		keys = keys[len(keys)-12:]
	}
	firstDisplayedMonth := monthByKey[keys[0]]

	for _, account := range utils.SortedKeys(flowsByAccount) {
		balance := decimal.Zero
		for _, key := range utils.SortedKeys(flowsByAccount[account]) {
			if monthByKey[key].Before(firstDisplayedMonth) {
				balance = balance.Add(flowsByAccount[account][key])
			}
		}

		for _, key := range keys {
			month := monthByKey[key]
			if firstMonthByAccount[account].After(month) {
				continue
			}

			balance = balance.Add(flowsByAccount[account][key])
			points = append(points, CheckingBalancePoint{Month: month, Account: account, Balance: balance})
		}
	}

	sort.Slice(points, func(i, j int) bool {
		if points[i].Month.Equal(points[j].Month) {
			return points[i].Account < points[j].Account
		}
		return points[i].Month.Before(points[j].Month)
	})

	return points
}

// GetAvailableMonthRange reports the first and last recorded posting month.
func GetAvailableMonthRange(db *gorm.DB) AvailableMonthRange {
	postings := query.Init(db).All()
	if len(postings) == 0 {
		return AvailableMonthRange{}
	}

	return AvailableMonthRange{
		From: utils.BeginningOfMonth(postings[0].Date),
		To:   utils.BeginningOfMonth(postings[len(postings)-1].Date),
	}
}

func GetCheckingBalance(db *gorm.DB) gin.H {
	return doGetBalance(db, "Assets:Checking:%", false)
}

func GetBalance(db *gorm.DB) gin.H {
	return doGetBalance(db, "Assets:%", true)
}

func doGetBalance(db *gorm.DB, pattern string, rollup bool) gin.H {
	postings := query.Init(db).Like(pattern, "Income:CapitalGains:%").All()
	postings = service.PopulateMarketPrice(db, postings)
	breakdowns := ComputeBreakdowns(db, postings, rollup)
	return gin.H{"asset_breakdowns": breakdowns}
}

func ComputeBreakdowns(db *gorm.DB, postings []posting.Posting, rollup bool) map[string]AssetBreakdown {
	accounts := make(map[string]bool)
	for _, p := range postings {
		if service.IsCapitalGains(p) {
			continue
		}

		if rollup {
			var parts []string
			for _, part := range strings.Split(p.Account, ":") {
				parts = append(parts, part)
				accounts[strings.Join(parts, ":")] = false
			}
		}
		accounts[p.Account] = true

	}

	result := make(map[string]AssetBreakdown)

	for group, leaf := range accounts {
		ps := lo.Filter(postings, func(p posting.Posting, _ int) bool {
			account := p.Account
			if service.IsCapitalGains(p) {
				account = service.CapitalGainsSourceAccount(p.Account)
			}
			return utils.IsSameOrParent(account, group)
		})
		result[group] = ComputeBreakdown(db, ps, leaf, group)
	}

	return result
}

func ComputeBreakdown(db *gorm.DB, ps []posting.Posting, leaf bool, group string) AssetBreakdown {
	investmentAmount := lo.Reduce(ps, func(acc decimal.Decimal, p posting.Posting, _ int) decimal.Decimal {
		if utils.IsCheckingAccount(p.Account) || p.Amount.LessThan(decimal.Zero) || service.IsInterest(db, p) || service.IsStockSplit(db, p) || service.IsCapitalGains(p) {
			return acc
		} else {
			return acc.Add(p.Amount)
		}
	}, decimal.Zero)
	withdrawalAmount := lo.Reduce(ps, func(acc decimal.Decimal, p posting.Posting, _ int) decimal.Decimal {
		if !service.IsCapitalGains(p) && (utils.IsCheckingAccount(p.Account) || p.Amount.GreaterThan(decimal.Zero) || service.IsInterest(db, p) || service.IsStockSplit(db, p)) {
			return acc
		} else {
			return acc.Add(p.Amount.Neg())
		}
	}, decimal.Zero)
	psWithoutCapitalGains := lo.Filter(ps, func(p posting.Posting, _ int) bool {
		return !service.IsCapitalGains(p)
	})
	marketAmount := accounting.CurrentBalance(psWithoutCapitalGains)
	var balanceUnits decimal.Decimal
	if leaf {
		balanceUnits = lo.Reduce(ps, func(acc decimal.Decimal, p posting.Posting, _ int) decimal.Decimal {
			if !utils.IsCurrency(p.Commodity) {
				return acc.Add(p.Quantity)
			}
			return decimal.Zero
		}, decimal.Zero)
	}

	xirr := service.XIRR(db, ps)
	netInvestment := investmentAmount.Sub(withdrawalAmount)
	gainAmount := marketAmount.Sub(netInvestment)
	absoluteReturn := decimal.Zero
	if !investmentAmount.IsZero() {
		absoluteReturn = marketAmount.Sub(netInvestment).Div(investmentAmount)
	}
	return AssetBreakdown{
		InvestmentAmount: investmentAmount,
		WithdrawalAmount: withdrawalAmount,
		MarketAmount:     marketAmount,
		XIRR:             xirr,
		Group:            group,
		BalanceUnits:     balanceUnits,
		GainAmount:       gainAmount,
		AbsoluteReturn:   absoluteReturn,
	}
}
