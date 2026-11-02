package server

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/samber/lo"
	"github.com/shopspring/decimal"

	"github.com/ananthakumaran/paisa/internal/accounting"
	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/ananthakumaran/paisa/internal/query"
	"github.com/ananthakumaran/paisa/internal/service"
	"github.com/ananthakumaran/paisa/internal/utils"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Aggregate struct {
	Date         time.Time       `json:"date"`
	Account      string          `json:"account"`
	MarketAmount decimal.Decimal `json:"market_amount"`
}

type AllocationTargetConfig struct {
	Name     string
	Target   decimal.Decimal
	Accounts []string
}

type AllocationTarget struct {
	Name           string               `json:"name"`
	Target         decimal.Decimal      `json:"target"`
	Current        decimal.Decimal      `json:"current"`
	Aggregates     map[string]Aggregate `json:"aggregates"`
	MarketAmount   decimal.Decimal      `json:"market_amount"`
	Drift          decimal.Decimal      `json:"drift"`
	DriftTolerance decimal.Decimal      `json:"drift_tolerance"`
	Status         string               `json:"status"`
	Accounts       []string             `json:"accounts"`
	Commodities    []string             `json:"commodities"`
}

type RebalanceItem struct {
	Name         string          `json:"name"`
	Target       decimal.Decimal `json:"target"`
	Current      decimal.Decimal `json:"current"`
	MarketAmount decimal.Decimal `json:"market_amount"`
	TargetAmount decimal.Decimal `json:"target_amount"`
	Action       string          `json:"action"`
	Amount       decimal.Decimal `json:"amount"`
	NewPercent   decimal.Decimal `json:"new_percent"`
}

type RebalancePlan struct {
	Mode        string          `json:"mode"`
	CashAmount  decimal.Decimal `json:"cash_amount"`
	TotalBefore decimal.Decimal `json:"total_before"`
	TotalAfter  decimal.Decimal `json:"total_after"`
	Items       []RebalanceItem `json:"items"`
}

type RebalanceRequest struct {
	Cash decimal.Decimal `json:"cash"`
	Mode string          `json:"mode"`
}

func GetAllocation(db *gorm.DB) gin.H {
	postings := query.Init(db).UntilToday().Like("Assets:%").All()

	now := utils.EndOfToday()
	postings = lo.Map(postings, func(p posting.Posting, _ int) posting.Posting {
		p.MarketAmount = service.GetMarketPrice(db, p, now)
		return p
	})
	aggregates := computeAggregate(db, postings, now)
	aggregates_timeline := computeAggregateTimeline(db, postings)
	allocation_targets := computeAllocationTargets(db, postings)
	res := gin.H{"aggregates": aggregates, "aggregates_timeline": aggregates_timeline, "allocation_targets": allocation_targets}
	if len(allocation_targets) > 0 {
		res["rebalance"] = ComputeRebalancePlan(allocation_targets, decimal.Zero, "full_rebalance")
	}
	return res
}

func computeAggregateTimeline(db *gorm.DB, postings []posting.Posting) []map[string]Aggregate {
	var timeline []map[string]Aggregate

	var p posting.Posting

	type RunningSum struct {
		units decimal.Decimal
		cost  decimal.Decimal
	}

	accumulator := make(map[string]map[string]RunningSum)

	if len(postings) == 0 {
		return timeline
	}

	end := utils.EndOfToday()
	for start := postings[0].Date; start.Before(end); start = start.AddDate(0, 0, 1) {
		for len(postings) > 0 && (postings[0].Date.Before(start) || postings[0].Date.Equal(start)) {
			p, postings = postings[0], postings[1:]
			rsByAccount := accumulator[p.Account]
			if rsByAccount == nil {
				rsByAccount = make(map[string]RunningSum)
			}

			rs := rsByAccount[p.Commodity]
			rs.units = rs.units.Add(p.Quantity)
			rs.cost = rs.cost.Add(p.Amount)

			rsByAccount[p.Commodity] = rs
			accumulator[p.Account] = rsByAccount

		}

		result := make(map[string]Aggregate)

		for account, rsByAccount := range accumulator {
			marketAmount := decimal.Zero

			for commodity, rs := range rsByAccount {
				if utils.IsCurrency(commodity) {
					marketAmount = marketAmount.Add(rs.cost)
				} else {
					price := service.GetUnitPrice(db, commodity, start)
					if !price.Value.Equal(decimal.Zero) {
						marketAmount = marketAmount.Add(rs.units.Mul(price.Value))
					} else {
						marketAmount = marketAmount.Add(rs.cost)
					}
				}
			}

			result[account] = Aggregate{Date: start, Account: account, MarketAmount: marketAmount}

		}

		timeline = append(timeline, result)

	}
	return timeline
}

func computeAllocationTargets(db *gorm.DB, postings []posting.Posting) []AllocationTarget {
	var targetAllocations []AllocationTarget
	allocationTargetConfigs := config.GetConfig().AllocationTargets

	if len(postings) == 0 || len(allocationTargetConfigs) == 0 {
		return targetAllocations
	}

	totalMarketAmount := accounting.CurrentBalance(postings)

	for _, allocationTargetConfig := range allocationTargetConfigs {
		targetAllocations = append(targetAllocations, computeAllocationTarget(db, postings, allocationTargetConfig, totalMarketAmount))
	}

	return targetAllocations
}

func computeAllocationTarget(db *gorm.DB, postings []posting.Posting, allocationTargetConfig config.AllocationTarget, total decimal.Decimal) AllocationTarget {
	date := utils.EndOfToday()
	filteredPostings := accounting.FilterByGlob(postings, allocationTargetConfig.Accounts)
	aggregates := computeAggregate(db, filteredPostings, date)
	currentTotal := accounting.CurrentBalance(filteredPostings)
	current := decimal.Zero
	if !total.IsZero() {
		current = currentTotal.Div(total).Mul(decimal.NewFromInt(100)).Round(2)
	}
	target := decimal.NewFromFloat(allocationTargetConfig.Target)
	drift := current.Sub(target)
	driftTolerance := decimal.NewFromFloat(5.0)
	if allocationTargetConfig.Drift > 0 {
		driftTolerance = decimal.NewFromFloat(allocationTargetConfig.Drift)
	}
	status := "in_band"
	if drift.GreaterThan(driftTolerance) {
		status = "overweight"
	} else if drift.LessThan(driftTolerance.Neg()) {
		status = "underweight"
	}

	return AllocationTarget{
		Name:           allocationTargetConfig.Name,
		Target:         target,
		Current:        current,
		Aggregates:     aggregates,
		MarketAmount:   currentTotal,
		Drift:          drift,
		DriftTolerance: driftTolerance,
		Status:         status,
		Accounts:       allocationTargetConfig.Accounts,
		Commodities:    []string{},
	}
}

func computeAggregate(db *gorm.DB, postings []posting.Posting, date time.Time) map[string]Aggregate {
	byAccount := lo.GroupBy(postings, func(p posting.Posting) string { return p.Account })
	result := make(map[string]Aggregate)
	for account, ps := range byAccount {
		var parts []string
		for part := range strings.SplitSeq(account, ":") {
			parts = append(parts, part)
			parent := strings.Join(parts, ":")
			result[parent] = Aggregate{Account: parent}
		}

		marketAmount := accounting.CurrentBalanceOn(db, ps, date)
		result[account] = Aggregate{Date: date, Account: account, MarketAmount: marketAmount}

	}
	return result
}

func allocationTargetPercent(target decimal.Decimal, totalTarget decimal.Decimal) decimal.Decimal {
	if totalTarget.IsZero() {
		return decimal.Zero
	}
	return target.Div(totalTarget).Mul(decimal.NewFromInt(100))
}

func ComputeRebalancePlan(targets []AllocationTarget, cash decimal.Decimal, mode string) RebalancePlan {
	if mode == "" {
		if cash.GreaterThan(decimal.Zero) {
			mode = "cash_injection"
		} else {
			mode = "full_rebalance"
		}
	}

	totalBefore := decimal.Zero
	totalTarget := decimal.Zero
	for _, t := range targets {
		totalBefore = totalBefore.Add(t.MarketAmount)
		totalTarget = totalTarget.Add(t.Target)
	}

	if cash.LessThan(decimal.Zero) {
		cash = decimal.Zero
	}

	totalAfter := totalBefore.Add(cash)
	items := make([]RebalanceItem, 0, len(targets))

	if totalAfter.IsZero() {
		for _, t := range targets {
			items = append(items, RebalanceItem{
				Name:         t.Name,
				Target:       t.Target,
				Current:      t.Current,
				MarketAmount: t.MarketAmount,
				TargetAmount: decimal.Zero,
				Action:       "HOLD",
				Amount:       decimal.Zero,
				NewPercent:   decimal.Zero,
			})
		}
		return RebalancePlan{
			Mode:        mode,
			CashAmount:  cash,
			TotalBefore: totalBefore,
			TotalAfter:  totalAfter,
			Items:       items,
		}
	}

	if mode == "full_rebalance" {
		for _, t := range targets {
			targetPercent := allocationTargetPercent(t.Target, totalTarget)
			idealTargetAmount := totalAfter.Mul(targetPercent).Div(decimal.NewFromInt(100)).Round(2)
			delta := idealTargetAmount.Sub(t.MarketAmount)
			action := "HOLD"
			amount := decimal.Zero

			if delta.GreaterThan(decimal.NewFromFloat(0.01)) {
				action = "BUY"
				amount = delta.Round(2)
			} else if delta.LessThan(decimal.NewFromFloat(-0.01)) {
				action = "SELL"
				amount = delta.Abs().Round(2)
			}

			items = append(items, RebalanceItem{
				Name:         t.Name,
				Target:       t.Target,
				Current:      t.Current,
				MarketAmount: t.MarketAmount,
				TargetAmount: idealTargetAmount,
				Action:       action,
				Amount:       amount,
				NewPercent:   targetPercent.Round(2),
			})
		}
	} else {
		type bucketDeficit struct {
			index   int
			deficit decimal.Decimal
			target  decimal.Decimal
		}
		deficits := make([]bucketDeficit, len(targets))
		totalDeficit := decimal.Zero

		for i, t := range targets {
			ideal := totalAfter.Mul(allocationTargetPercent(t.Target, totalTarget)).Div(decimal.NewFromInt(100)).Round(2)
			def := ideal.Sub(t.MarketAmount)
			if def.LessThan(decimal.Zero) {
				def = decimal.Zero
			}
			deficits[i] = bucketDeficit{index: i, deficit: def, target: t.Target}
			totalDeficit = totalDeficit.Add(def)
		}

		allocations := make([]decimal.Decimal, len(targets))
		if totalDeficit.GreaterThan(decimal.Zero) {
			if cash.LessThanOrEqual(totalDeficit) {
				for i, d := range deficits {
					if totalDeficit.GreaterThan(decimal.Zero) {
						allocations[i] = cash.Mul(d.deficit).Div(totalDeficit).Round(2)
					}
				}
			} else {
				remaining := cash.Sub(totalDeficit)
				for i, d := range deficits {
					alloc := d.deficit
					if totalTarget.GreaterThan(decimal.Zero) {
						surplus := remaining.Mul(d.target).Div(totalTarget).Round(2)
						alloc = alloc.Add(surplus)
					}
					allocations[i] = alloc
				}
			}
		} else {
			for i, t := range targets {
				if totalTarget.GreaterThan(decimal.Zero) {
					allocations[i] = cash.Mul(t.Target).Div(totalTarget).Round(2)
				}
			}
		}

		for i, t := range targets {
			alloc := allocations[i]
			action := "HOLD"
			if alloc.GreaterThan(decimal.NewFromFloat(0.01)) {
				action = "BUY"
			} else {
				alloc = decimal.Zero
			}

			newAmount := t.MarketAmount.Add(alloc)
			newPercent := decimal.Zero
			if totalAfter.GreaterThan(decimal.Zero) {
				newPercent = newAmount.Div(totalAfter).Mul(decimal.NewFromInt(100)).Round(2)
			}
			targetAmount := totalAfter.Mul(allocationTargetPercent(t.Target, totalTarget)).Div(decimal.NewFromInt(100)).Round(2)

			items = append(items, RebalanceItem{
				Name:         t.Name,
				Target:       t.Target,
				Current:      t.Current,
				MarketAmount: t.MarketAmount,
				TargetAmount: targetAmount,
				Action:       action,
				Amount:       alloc,
				NewPercent:   newPercent,
			})
		}
	}

	return RebalancePlan{
		Mode:        mode,
		CashAmount:  cash,
		TotalBefore: totalBefore,
		TotalAfter:  totalAfter,
		Items:       items,
	}
}

func RebalanceHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req RebalanceRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}

		switch req.Mode {
		case "", "cash_injection", "full_rebalance":
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "mode must be cash_injection or full_rebalance"})
			return
		}

		postings := query.Init(db).UntilToday().Like("Assets:%").All()
		now := utils.EndOfToday()
		postings = lo.Map(postings, func(p posting.Posting, _ int) posting.Posting {
			p.MarketAmount = service.GetMarketPrice(db, p, now)
			return p
		})
		targets := computeAllocationTargets(db, postings)
		plan := ComputeRebalancePlan(targets, req.Cash, req.Mode)
		c.JSON(http.StatusOK, plan)
	}
}

func validateAllocationTargets(targets []config.AllocationTarget) error {
	if len(targets) == 0 {
		return nil
	}

	total := decimal.Zero
	for _, t := range targets {
		if strings.TrimSpace(t.Name) == "" {
			return fmt.Errorf("target name cannot be empty")
		}
		if t.Target < 0 || t.Target > 100 {
			return fmt.Errorf("target percentage must be between 0 and 100")
		}
		if t.Drift < 0 || t.Drift > 100 {
			return fmt.Errorf("drift tolerance must be between 0 and 100")
		}
		if len(t.Accounts) == 0 {
			return fmt.Errorf("target %q must have at least one account", t.Name)
		}
		total = total.Add(decimal.NewFromFloat(t.Target))
	}

	if !total.Round(2).Equal(decimal.NewFromInt(100)) {
		return fmt.Errorf("target percentages must sum to 100, got %s", total)
	}

	return nil
}

func SaveAllocationTargetsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		if config.GetConfig().Readonly {
			c.JSON(http.StatusOK, gin.H{"success": true})
			return
		}

		var targets []config.AllocationTarget
		if err := c.ShouldBindJSON(&targets); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid target data: " + err.Error()})
			return
		}

		if err := validateAllocationTargets(targets); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		cfg := config.GetConfig()
		cfg.AllocationTargets = targets
		if err := config.SaveConfigObject(cfg); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save configuration: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true})
	}
}
