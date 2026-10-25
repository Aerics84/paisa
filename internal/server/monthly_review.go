package server

import (
	"sort"
	"strings"
	"time"

	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/ananthakumaran/paisa/internal/model/transaction"
	"github.com/ananthakumaran/paisa/internal/query"
	"github.com/ananthakumaran/paisa/internal/service"
	"github.com/ananthakumaran/paisa/internal/utils"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

const (
	MonthlyActivityReady           = "ready"
	MonthlyActivityNone            = "no_recorded_activity"
	MonthlyPerformanceReady        = "ready"
	MonthlyPerformanceNotAvailable = "not_available"
)

// MonthlyMetricPoint is one point of the selected month's short trend. A
// missing month stays unavailable instead of being rendered as a real zero.
type MonthlyMetricPoint struct {
	Month         string          `json:"month"`
	Value         decimal.Decimal `json:"value"`
	Available     bool            `json:"available"`
	IsMonthToDate bool            `json:"is_month_to_date"`
}

// MonthlyReview is the period-specific counterpart to the live snapshot. It
// never mixes live balances into the selected month.
type MonthlyReview struct {
	Month                    string                       `json:"month"`
	ActivityState            string                       `json:"activity_state"`
	PeriodComplete           bool                         `json:"period_complete"`
	NetIncomeAfterTax        decimal.Decimal              `json:"net_income_after_tax"`
	NetTax                   decimal.Decimal              `json:"net_tax"`
	NetExpense               decimal.Decimal              `json:"net_expense"`
	Surplus                  decimal.Decimal              `json:"surplus"`
	HasIncomePostings        bool                         `json:"has_income_postings"`
	HasExpensePostings       bool                         `json:"has_expense_postings"`
	FlowGraph                Graph                        `json:"flow_graph"`
	NetIncomeTrend           []MonthlyMetricPoint         `json:"net_income_trend"`
	InvestmentContributions  decimal.Decimal              `json:"investment_contributions"`
	InvestmentMarketMovement decimal.Decimal              `json:"investment_market_movement"`
	DividendIncome           decimal.Decimal              `json:"dividend_income"`
	BestPerformers           []MonthlyPositionPerformance `json:"best_performers"`
	WeakestPerformers        []MonthlyPositionPerformance `json:"weakest_performers"`
	PerformanceState         string                       `json:"performance_state"`
	PerformanceAsOf          time.Time                    `json:"performance_as_of"`
}

func emptyFlowGraph() Graph {
	return Graph{Nodes: []Node{}, Links: []Link{}}
}

func monthlyIncome(postings []posting.Posting) (decimal.Decimal, decimal.Decimal, decimal.Decimal) {
	grossSalary := decimal.Zero
	grossOther := decimal.Zero
	netTax := decimal.Zero

	for _, p := range postings {
		switch {
		case strings.HasPrefix(p.Account, "Income:Salary"):
			grossSalary = grossSalary.Sub(p.Amount)
		case utils.FirstName(p.Account) == "Income":
			grossOther = grossOther.Sub(p.Amount)
		case utils.IsSameOrParent(p.Account, "Expenses:Tax"):
			netTax = netTax.Add(p.Amount)
		}
	}

	return grossSalary, grossOther, netTax
}

func monthlyExpense(postings []posting.Posting) decimal.Decimal {
	total := decimal.Zero
	for _, p := range postings {
		if utils.FirstName(p.Account) == "Expenses" && !utils.IsSameOrParent(p.Account, "Expenses:Tax") {
			total = total.Add(p.Amount)
		}
	}
	return total
}

func monthlyInvestmentContributions(postings []posting.Posting) decimal.Decimal {
	total := decimal.Zero
	for _, p := range postings {
		if utils.FirstName(p.Account) == "Assets" && !utils.IsCheckingAccount(p.Account) {
			total = total.Add(p.Amount)
		}
	}
	return total
}

func monthlyDividendIncome(postings []posting.Posting) decimal.Decimal {
	total := decimal.Zero
	for _, p := range postings {
		if utils.IsSameOrParent(p.Account, "Income:Dividend") {
			total = total.Sub(p.Amount)
		}
	}
	return total
}

func monthlyIncomeAvailable(postings []posting.Posting) bool {
	for _, p := range postings {
		if utils.FirstName(p.Account) == "Income" || utils.IsSameOrParent(p.Account, "Expenses:Tax") {
			return true
		}
	}
	return false
}

func computeMonthlyIncomeTrend(db *gorm.DB, selectedStart time.Time, currentMonth time.Time) []MonthlyMetricPoint {
	points := make([]MonthlyMetricPoint, 0, 3)

	for offset := 2; offset >= 0; offset-- {
		start := selectedStart.AddDate(0, -offset, 0)
		end := utils.EndOfMonth(start)
		if start.Equal(currentMonth) {
			end = utils.EndOfToday()
		}

		postings := query.Init(db).Where("substr(date, 1, 10) >= ? and substr(date, 1, 10) <= ?", start.Format("2006-01-02"), end.Format("2006-01-02")).All()
		point := MonthlyMetricPoint{
			Month:         start.Format("2006-01"),
			Value:         decimal.Zero,
			Available:     monthlyIncomeAvailable(postings),
			IsMonthToDate: start.Equal(currentMonth),
		}

		if point.Available {
			grossSalary, grossOther, netTax := monthlyIncome(postings)
			point.Value = grossSalary.Add(grossOther).Sub(netTax)
		}

		points = append(points, point)
	}

	return points
}

type monthlyPositionCandidate struct {
	account   string
	commodity string
	postings  []posting.Posting
}

func candidatePositions(postings []posting.Posting) ([]monthlyPositionCandidate, bool) {
	byAccount := make(map[string][]posting.Posting)
	order := make([]string, 0)
	for _, p := range postings {
		if _, found := byAccount[p.Account]; !found {
			order = append(order, p.Account)
		}
		byAccount[p.Account] = append(byAccount[p.Account], p)
	}

	var candidates []monthlyPositionCandidate
	complete := true
	for _, account := range order {
		ps := byAccount[account]
		commodities := make(map[string]bool)
		for _, p := range ps {
			if !utils.IsCurrency(p.Commodity) {
				commodities[p.Commodity] = true
			}
		}

		if len(commodities) == 0 {
			continue
		}
		if len(commodities) > 1 {
			// A single account holding multiple priced commodities cannot be
			// valued as one comparable position. Do not hide it from coverage.
			complete = false
			continue
		}

		var commodity string
		for name := range commodities {
			commodity = name
		}
		candidates = append(candidates, monthlyPositionCandidate{account: account, commodity: commodity, postings: ps})
	}

	return candidates, complete
}

func accountLeaf(account string) string {
	parts := strings.Split(account, ":")
	return parts[len(parts)-1]
}

func dividendFlowsByAccount(monthPostings []posting.Posting, candidates []monthlyPositionCandidate) map[string][]PositionCashFlow {
	byLeaf := make(map[string][]string)
	for _, candidate := range candidates {
		leaf := accountLeaf(candidate.account)
		byLeaf[leaf] = append(byLeaf[leaf], candidate.account)
	}

	flows := make(map[string][]PositionCashFlow)
	for _, p := range monthPostings {
		if !utils.IsSameOrParent(p.Account, "Income:Dividend") || !strings.Contains(p.Account, ":") {
			continue
		}

		suffix := strings.TrimPrefix(p.Account, "Income:Dividend:")
		if suffix == p.Account || suffix == "" {
			continue
		}

		leaf := suffix
		if index := strings.LastIndex(suffix, ":"); index >= 0 {
			leaf = suffix[index+1:]
		}

		accounts := byLeaf[leaf]
		if len(accounts) != 1 || !p.Amount.LessThan(decimal.Zero) {
			continue
		}

		flows[accounts[0]] = append(flows[accounts[0]], PositionCashFlow{
			Date:   p.Date,
			Kind:   PositionCashFlowDividend,
			Amount: p.Amount.Neg(),
		})
	}

	return flows
}

func splitAffectedAccounts(monthPostings []posting.Posting) map[string]bool {
	affected := make(map[string]bool)
	for _, t := range transaction.Build(monthPostings) {
		if !isQuantityOnlyTransaction(t) {
			continue
		}
		for _, p := range t.Postings {
			affected[p.Account] = true
		}
	}
	return affected
}

func positionFlows(monthPostings []posting.Posting, account string, commodity string) []PositionCashFlow {
	flows := make([]PositionCashFlow, 0)
	for _, p := range monthPostings {
		if p.Account != account || p.Commodity != commodity {
			continue
		}

		if p.Amount.GreaterThan(decimal.Zero) {
			flows = append(flows, PositionCashFlow{Date: p.Date, Kind: PositionCashFlowPurchase, Amount: p.Amount})
		} else if p.Amount.LessThan(decimal.Zero) {
			flows = append(flows, PositionCashFlow{Date: p.Date, Kind: PositionCashFlowSale, Amount: p.Amount.Neg()})
		}
	}
	return flows
}

func positionStartValue(db *gorm.DB, candidate monthlyPositionCandidate, periodStart time.Time) (decimal.Decimal, time.Time, bool) {
	quantity := decimal.Zero
	for _, p := range candidate.postings {
		if p.Commodity != candidate.commodity {
			continue
		}
		if p.Date.Before(periodStart) {
			quantity = quantity.Add(p.Quantity)
		}
	}

	if quantity.LessThan(decimal.Zero) {
		return decimal.Zero, time.Time{}, false
	}
	if quantity.IsZero() {
		return decimal.Zero, time.Time{}, true
	}

	// The prior close, not a quote dated on PeriodStart itself: a quote on the
	// first day belongs to that day's end-of-day valuation.
	quote, found := service.FindUnitPrice(db, candidate.commodity, periodStart.Add(-time.Nanosecond))
	if !found || !quote.Value.GreaterThan(decimal.Zero) {
		return decimal.Zero, time.Time{}, false
	}

	return quantity.Mul(quote.Value), quote.Date, true
}

func computePositionValuations(db *gorm.DB, candidate monthlyPositionCandidate, flows []PositionCashFlow, periodStart time.Time, periodEnd time.Time) ([]PositionValuation, map[time.Time]bool, bool) {
	flowDates := make(map[time.Time]bool)
	for _, flow := range flows {
		flowDates[normalizePerformanceDate(flow.Date)] = true
	}

	valuations := make([]PositionValuation, 0)
	holdings := make(map[time.Time]bool)
	quantity := decimal.Zero
	index := 0
	sorted := make([]posting.Posting, 0, len(candidate.postings))
	for _, p := range candidate.postings {
		if p.Commodity == candidate.commodity {
			sorted = append(sorted, p)
		}
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Date.Before(sorted[j].Date) })

	for day := periodStart; !day.After(periodEnd); day = day.AddDate(0, 0, 1) {
		for index < len(sorted) && !sorted[index].Date.After(day) {
			quantity = quantity.Add(sorted[index].Quantity)
			index++
		}

		dayKey := normalizePerformanceDate(day)
		valuation := PositionValuation{Date: day}

		if quantity.LessThan(decimal.Zero) {
			return nil, nil, false
		}

		hasFlow := flowDates[dayKey]
		if quantity.IsZero() && !hasFlow {
			valuations = append(valuations, valuation)
			continue
		}

		quote, found := service.FindUnitPrice(db, candidate.commodity, day)
		if !found || !quote.Value.GreaterThan(decimal.Zero) {
			return nil, nil, false
		}

		valuation.MarketValue = quantity.Mul(quote.Value)
		valuation.PriceDate = quote.Date
		valuations = append(valuations, valuation)
		if quantity.GreaterThan(decimal.Zero) {
			holdings[dayKey] = true
		}
	}

	return valuations, holdings, true
}

func positionEndValue(db *gorm.DB, candidate monthlyPositionCandidate, periodEnd time.Time) (decimal.Decimal, bool) {
	quantity := decimal.Zero
	for _, p := range candidate.postings {
		if p.Commodity != candidate.commodity {
			continue
		}
		if !p.Date.After(periodEnd) {
			quantity = quantity.Add(p.Quantity)
		}
	}

	if quantity.LessThan(decimal.Zero) {
		return decimal.Zero, false
	}
	if quantity.IsZero() {
		return decimal.Zero, true
	}

	quote, found := service.FindUnitPrice(db, candidate.commodity, periodEnd)
	if !found || !quote.Value.GreaterThan(decimal.Zero) {
		return decimal.Zero, false
	}

	return quantity.Mul(quote.Value), true
}

func computeMonthlyPerformance(db *gorm.DB, monthPostings []posting.Posting, periodStart time.Time, periodEnd time.Time) ([]MonthlyPositionPerformance, []MonthlyPositionPerformance, string, time.Time, decimal.Decimal) {
	assetPostings := query.Init(db).Where("substr(date, 1, 10) <= ?", periodEnd.Format("2006-01-02")).Like("Assets:%").NotAccountPrefix("Assets:Checking").All()
	candidates, coverageComplete := candidatePositions(assetPostings)
	dividendFlows := dividendFlowsByAccount(monthPostings, candidates)
	splits := splitAffectedAccounts(monthPostings)

	var ranked []MonthlyPositionPerformance
	totalValue := decimal.Zero
	rankableValue := decimal.Zero
	marketMovement := decimal.Zero
	var performanceAsOf time.Time

	for _, candidate := range candidates {
		endValue, priced := positionEndValue(db, candidate, periodEnd)
		if !priced {
			coverageComplete = false
		}
		totalValue = totalValue.Add(endValue)

		if splits[candidate.account] {
			continue
		}

		startValue, startPriceDate, startPriced := positionStartValue(db, candidate, periodStart)
		if !startPriced {
			coverageComplete = false
			continue
		}

		flows := positionFlows(monthPostings, candidate.account, candidate.commodity)
		valuations, holdings, ok := computePositionValuations(db, candidate, flows, periodStart, periodEnd)
		if !ok {
			coverageComplete = false
			continue
		}

		// A dividend is only attributable while the position is held.
		heldDividends := make([]PositionCashFlow, 0)
		for _, flow := range dividendFlows[candidate.account] {
			if holdings[normalizePerformanceDate(flow.Date)] {
				heldDividends = append(heldDividends, flow)
			}
		}

		cashFlows := append(append([]PositionCashFlow(nil), flows...), heldDividends...)
		performance, rankable := calculateMonthlyPositionPerformance(PositionMonthlyInput{
			Account:         candidate.account,
			PeriodStart:     periodStart,
			PeriodEnd:       periodEnd,
			StartValue:      startValue,
			StartPriceDate:  startPriceDate,
			DailyValuations: valuations,
			CashFlows:       cashFlows,
			MaxPriceAgeDays: DefaultMaxPriceAgeDays,
		})
		if !rankable {
			continue
		}

		ranked = append(ranked, performance)
		rankableValue = rankableValue.Add(endValue)
		marketMovement = marketMovement.Add(performance.GainLoss.Sub(performance.DividendIncome))
		if performanceAsOf.IsZero() || performance.PriceAsOf.Before(performanceAsOf) {
			performanceAsOf = performance.PriceAsOf
		}
	}

	if !coverageComplete || len(ranked) < 2 || totalValue.LessThanOrEqual(decimal.Zero) ||
		rankableValue.Mul(decimal.NewFromInt(100)).LessThan(totalValue.Mul(decimal.NewFromInt(50))) {
		return []MonthlyPositionPerformance{}, []MonthlyPositionPerformance{}, MonthlyPerformanceNotAvailable, time.Time{}, decimal.Zero
	}

	sort.SliceStable(ranked, func(i, j int) bool {
		return ranked[i].TotalReturnPercent.GreaterThan(ranked[j].TotalReturnPercent)
	})

	best := ranked
	if len(best) > 3 {
		best = best[:3]
	}

	weakest := make([]MonthlyPositionPerformance, 0, 3)
	for index := len(ranked) - 1; index >= 0 && len(weakest) < 3; index-- {
		weakest = append(weakest, ranked[index])
	}

	return best, weakest, MonthlyPerformanceReady, performanceAsOf, marketMovement
}

// GetMonthlyReview derives every period-specific number from postings in the
// selected calendar month. Current balances are deliberately not part of it.
func GetMonthlyReview(db *gorm.DB, month time.Time) MonthlyReview {
	periodStart := utils.BeginningOfMonth(month)
	currentMonth := utils.BeginningOfMonth(utils.Now())
	todayEnd := utils.EndOfToday()

	periodEnd := utils.EndOfMonth(periodStart)
	periodComplete := true
	if periodStart.Equal(currentMonth) {
		periodEnd = todayEnd
		periodComplete = false
	} else if periodEnd.After(todayEnd) {
		periodEnd = todayEnd
		periodComplete = false
	}

	postings := query.Init(db).Where("substr(date, 1, 10) >= ? and substr(date, 1, 10) <= ?", periodStart.Format("2006-01-02"), periodEnd.Format("2006-01-02")).All()

	review := MonthlyReview{
		Month:             periodStart.Format("2006-01"),
		ActivityState:     MonthlyActivityNone,
		PeriodComplete:    periodComplete,
		FlowGraph:         emptyFlowGraph(),
		NetIncomeTrend:    computeMonthlyIncomeTrend(db, periodStart, currentMonth),
		BestPerformers:    []MonthlyPositionPerformance{},
		WeakestPerformers: []MonthlyPositionPerformance{},
		PerformanceState:  MonthlyPerformanceNotAvailable,
	}

	if len(postings) == 0 {
		review.ActivityState = MonthlyActivityNone
	} else {
		review.ActivityState = MonthlyActivityReady

		grossSalary, grossOther, netTax := monthlyIncome(postings)
		review.NetTax = netTax
		review.NetIncomeAfterTax = grossSalary.Add(grossOther).Sub(netTax)
		review.NetExpense = monthlyExpense(postings)
		review.Surplus = review.NetIncomeAfterTax.Sub(review.NetExpense)
		review.InvestmentContributions = monthlyInvestmentContributions(postings)
		review.DividendIncome = monthlyDividendIncome(postings)
		for _, p := range postings {
			if utils.FirstName(p.Account) == "Income" {
				review.HasIncomePostings = true
			}
			if utils.FirstName(p.Account) == "Expenses" {
				review.HasExpensePostings = true
			}
		}

		graph := sortGraph(computeHierarchyGraph(postings))
		if len(graph.Nodes) == 0 && len(graph.Links) == 0 {
			graph = emptyFlowGraph()
		}
		review.FlowGraph = graph
	}

	// Held positions can move without any posting in the month, so performance
	// is derived independently from the ledger-activity state.
	best, weakest, performanceState, performanceAsOf, marketMovement := computeMonthlyPerformance(db, postings, periodStart, periodEnd)
	review.BestPerformers = best
	review.WeakestPerformers = weakest
	review.PerformanceState = performanceState
	review.PerformanceAsOf = performanceAsOf
	review.InvestmentMarketMovement = marketMovement

	return review
}
