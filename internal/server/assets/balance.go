package assets

import (
	"sort"
	"strings"
	"time"

	"github.com/samber/lo"
	"github.com/shopspring/decimal"

	"github.com/ananthakumaran/paisa/internal/accounting"
	"github.com/ananthakumaran/paisa/internal/model/commodity"
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
	Commodity        string          `json:"commodity,omitempty"`
	CommodityName    string          `json:"commodity_name,omitempty"`
	CommodityCode    string          `json:"commodity_code,omitempty"`
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

	flowsByAccount := make(map[string]map[string]decimal.Decimal)
	firstMonthByAccount := make(map[string]time.Time)

	for _, p := range postings {
		month := utils.BeginningOfMonth(p.Date)
		key := month.Format("2006-01")

		if flowsByAccount[p.Account] == nil {
			flowsByAccount[p.Account] = make(map[string]decimal.Decimal)
		}
		flowsByAccount[p.Account][key] = flowsByAccount[p.Account][key].Add(p.Amount)

		if first, found := firstMonthByAccount[p.Account]; !found || month.Before(first) {
			firstMonthByAccount[p.Account] = month
		}
	}

	keys := make([]string, 0, 12)
	lastMonth := utils.BeginningOfMonth(postings[len(postings)-1].Date)
	windowStart := lastMonth.AddDate(0, -11, 0)
	monthByKey := make(map[string]time.Time, len(keys))
	for index := 0; index < 12; index++ {
		month := windowStart.AddDate(0, index, 0)
		key := month.Format("2006-01")
		keys = append(keys, key)
		monthByKey[key] = month
	}

	for _, account := range utils.SortedKeys(flowsByAccount) {
		balance := decimal.Zero
		for _, key := range utils.SortedKeys(flowsByAccount[account]) {
			if key < keys[0] {
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

// GetAvailableMonthRange reports the first and last selectable month. The end
// is never earlier than the current month, so a running month with no postings
// can still show its month-to-date review.
func GetAvailableMonthRange(db *gorm.DB) AvailableMonthRange {
	postings := query.Init(db).All()
	if len(postings) == 0 {
		return AvailableMonthRange{}
	}

	to := utils.BeginningOfMonth(postings[len(postings)-1].Date)
	current := utils.BeginningOfMonth(utils.Now())
	if current.After(to) {
		to = current
	}

	return AvailableMonthRange{
		From: utils.BeginningOfMonth(postings[0].Date),
		To:   to,
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

func commodityMetadata(ps []posting.Posting) (string, string, string) {
	names := map[string]bool{}
	for _, p := range ps {
		if !utils.IsCurrency(p.Commodity) {
			names[p.Commodity] = true
		}
	}
	if len(names) != 1 {
		return "", "", ""
	}
	var name string
	for candidate := range names {
		name = candidate
	}
	configured := commodity.FindByName(name)
	if configured.Name == "" {
		return "", "", ""
	}
	return name, configured.Name, configured.Price.Code
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
	commodityName, commodityLabel, commodityCode := "", "", ""
	if leaf {
		commodityName, commodityLabel, commodityCode = commodityMetadata(ps)
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
		Commodity:        commodityName,
		CommodityName:    commodityLabel,
		CommodityCode:    commodityCode,
	}
}
