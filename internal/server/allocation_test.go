package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/model/cache"
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/ananthakumaran/paisa/internal/model/price"
	"github.com/ananthakumaran/paisa/internal/service"
	"github.com/ananthakumaran/paisa/internal/utils"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openFuturePostingsTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	require.NoError(t, config.LoadConfig([]byte(`
journal_path: main.ledger
db_path: paisa.db
default_currency: EUR
time_zone: UTC
`), ""))

	name := strings.NewReplacer("/", "_", "\\", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&posting.Posting{}, &cache.Cache{}, &price.Price{}))

	service.ClearInterestCache()
	service.ClearPriceCache()
	t.Cleanup(service.ClearInterestCache)
	t.Cleanup(service.ClearPriceCache)

	return db
}

func addFuturePostingTestPosting(t *testing.T, db *gorm.DB, transactionID string, date time.Time, account string, amount int64) {
	t.Helper()

	require.NoError(t, db.Create(&posting.Posting{
		TransactionID: transactionID,
		Date:          date,
		Payee:         transactionID,
		Account:       account,
		Commodity:     "EUR",
		Quantity:      decimal.NewFromInt(amount),
		Amount:        decimal.NewFromInt(amount),
	}).Error)
}

func futurePostingsTestDates() (time.Time, time.Time) {
	today := utils.BeginningOfDay(utils.Now())
	return today, today.AddDate(0, 1, 0)
}

func TestAllocationExcludesFuturePostings(t *testing.T) {
	db := openFuturePostingsTestDB(t)

	today, future := futurePostingsTestDates()
	addFuturePostingTestPosting(t, db, "today", today, "Assets:Broker", 1000)
	addFuturePostingTestPosting(t, db, "future", future, "Assets:Broker", 500)

	result := GetAllocation(db)
	aggregates, ok := result["aggregates"].(map[string]Aggregate)
	require.True(t, ok)

	total := decimal.Zero
	for _, aggregate := range aggregates {
		total = total.Add(aggregate.MarketAmount)
	}
	require.True(t, total.Equal(decimal.NewFromInt(1000)), "future postings must not inflate the allocation, got %s", total)
}

func allocationTestPosting(account string, amount int64) posting.Posting {
	value := decimal.NewFromInt(amount)
	return posting.Posting{
		Account:      account,
		Commodity:    "EUR",
		Quantity:     value,
		Amount:       value,
		MarketAmount: value,
		Date:         utils.EndOfToday(),
	}
}

func TestAllocationTargetDriftStatus(t *testing.T) {
	db := openFuturePostingsTestDB(t)
	postings := []posting.Posting{
		allocationTestPosting("Assets:Equity:Stock", 7000),
		allocationTestPosting("Assets:Debt:Fund", 3000),
	}
	total := decimal.NewFromInt(10000)

	equity := computeAllocationTarget(db, postings, config.AllocationTarget{
		Name:     "Equity",
		Target:   60,
		Drift:    5,
		Accounts: []string{"Assets:Equity:*"},
	}, total)
	debt := computeAllocationTarget(db, postings, config.AllocationTarget{
		Name:     "Debt",
		Target:   40,
		Drift:    5,
		Accounts: []string{"Assets:Debt:*"},
	}, total)

	assert.True(t, equity.Current.Equal(decimal.NewFromInt(70)), "current equity: %s", equity.Current)
	assert.True(t, equity.Drift.Equal(decimal.NewFromInt(10)), "equity drift: %s", equity.Drift)
	assert.Equal(t, "overweight", equity.Status)
	assert.True(t, equity.MarketAmount.Equal(decimal.NewFromInt(7000)))

	assert.True(t, debt.Current.Equal(decimal.NewFromInt(30)), "current debt: %s", debt.Current)
	assert.True(t, debt.Drift.Equal(decimal.NewFromInt(-10)), "debt drift: %s", debt.Drift)
	assert.Equal(t, "underweight", debt.Status)
	assert.True(t, debt.MarketAmount.Equal(decimal.NewFromInt(3000)))
}

func TestAllocationTargetWithinTolerance(t *testing.T) {
	db := openFuturePostingsTestDB(t)
	postings := []posting.Posting{
		allocationTestPosting("Assets:Equity:Stock", 6200),
		allocationTestPosting("Assets:Debt:Fund", 3800),
	}
	total := decimal.NewFromInt(10000)

	equity := computeAllocationTarget(db, postings, config.AllocationTarget{
		Name:     "Equity",
		Target:   60,
		Drift:    5,
		Accounts: []string{"Assets:Equity:*"},
	}, total)

	assert.True(t, equity.Current.Equal(decimal.NewFromInt(62)), "current equity: %s", equity.Current)
	assert.True(t, equity.Drift.Equal(decimal.NewFromInt(2)), "equity drift: %s", equity.Drift)
	assert.True(t, equity.DriftTolerance.Equal(decimal.NewFromInt(5)))
	assert.Equal(t, "in_band", equity.Status)
}

func TestComputeRebalancePlanCashInjection(t *testing.T) {
	targets := []AllocationTarget{
		{
			Name:         "Equity",
			Target:       decimal.NewFromInt(60),
			Current:      decimal.NewFromInt(70),
			MarketAmount: decimal.NewFromInt(7000),
		},
		{
			Name:         "Debt",
			Target:       decimal.NewFromInt(40),
			Current:      decimal.NewFromInt(30),
			MarketAmount: decimal.NewFromInt(3000),
		},
	}

	plan := ComputeRebalancePlan(targets, decimal.NewFromInt(1000), "cash_injection")

	assert.Equal(t, "cash_injection", plan.Mode)
	assert.True(t, plan.TotalBefore.Equal(decimal.NewFromInt(10000)))
	assert.True(t, plan.TotalAfter.Equal(decimal.NewFromInt(11000)))
	require.Len(t, plan.Items, 2)

	assert.Equal(t, "Equity", plan.Items[0].Name)
	assert.Equal(t, "HOLD", plan.Items[0].Action)
	assert.True(t, plan.Items[0].Amount.Equal(decimal.Zero))
	assert.True(t, plan.Items[0].NewPercent.Equal(decimal.NewFromFloat(63.64)))

	assert.Equal(t, "Debt", plan.Items[1].Name)
	assert.Equal(t, "BUY", plan.Items[1].Action)
	assert.True(t, plan.Items[1].Amount.Equal(decimal.NewFromInt(1000)))

	bought := decimal.Zero
	sold := decimal.Zero
	held := decimal.Zero
	for _, item := range plan.Items {
		switch item.Action {
		case "BUY":
			bought = bought.Add(item.Amount)
		case "SELL":
			sold = sold.Add(item.Amount)
		case "HOLD":
			held = held.Add(item.Amount)
		}
	}
	assert.True(t, bought.Equal(decimal.NewFromInt(1000)), "bought: %s", bought)
	assert.True(t, sold.Equal(decimal.Zero), "sold: %s", sold)
	assert.True(t, held.Equal(decimal.Zero), "held: %s", held)
}

func TestComputeRebalancePlanFullRebalance(t *testing.T) {
	targets := []AllocationTarget{
		{
			Name:         "Equity",
			Target:       decimal.NewFromInt(60),
			Current:      decimal.NewFromInt(70),
			MarketAmount: decimal.NewFromInt(7000),
		},
		{
			Name:         "Debt",
			Target:       decimal.NewFromInt(40),
			Current:      decimal.NewFromInt(30),
			MarketAmount: decimal.NewFromInt(3000),
		},
	}

	plan := ComputeRebalancePlan(targets, decimal.Zero, "full_rebalance")

	assert.Equal(t, "full_rebalance", plan.Mode)
	assert.True(t, plan.TotalBefore.Equal(decimal.NewFromInt(10000)))
	assert.True(t, plan.TotalAfter.Equal(decimal.NewFromInt(10000)))
	require.Len(t, plan.Items, 2)

	assert.Equal(t, "SELL", plan.Items[0].Action)
	assert.True(t, plan.Items[0].Amount.Equal(decimal.NewFromInt(1000)))
	assert.True(t, plan.Items[0].TargetAmount.Equal(decimal.NewFromInt(6000)))

	assert.Equal(t, "BUY", plan.Items[1].Action)
	assert.True(t, plan.Items[1].Amount.Equal(decimal.NewFromInt(1000)))
	assert.True(t, plan.Items[1].TargetAmount.Equal(decimal.NewFromInt(4000)))

	bought := decimal.Zero
	sold := decimal.Zero
	for _, item := range plan.Items {
		switch item.Action {
		case "BUY":
			bought = bought.Add(item.Amount)
		case "SELL":
			sold = sold.Add(item.Amount)
		}
	}
	assert.True(t, bought.Equal(sold), "buys and sells must net to zero, bought: %s, sold: %s", bought, sold)
}

func TestComputeRebalancePlanCashInjectionZeroCash(t *testing.T) {
	targets := []AllocationTarget{
		{
			Name:         "Equity",
			Target:       decimal.NewFromInt(60),
			Current:      decimal.NewFromInt(70),
			MarketAmount: decimal.NewFromInt(7000),
		},
		{
			Name:         "Debt",
			Target:       decimal.NewFromInt(40),
			Current:      decimal.NewFromInt(30),
			MarketAmount: decimal.NewFromInt(3000),
		},
	}

	plan := ComputeRebalancePlan(targets, decimal.Zero, "cash_injection")

	assert.Equal(t, "cash_injection", plan.Mode)
	require.Len(t, plan.Items, 2)
	for _, item := range plan.Items {
		assert.Equal(t, "HOLD", item.Action, "zero cash injection must not buy or sell")
		assert.True(t, item.Amount.Equal(decimal.Zero))
	}
}

func TestComputeRebalancePlanFullRebalanceNonHundredTargets(t *testing.T) {
	targets := []AllocationTarget{
		{
			Name:         "Equity",
			Target:       decimal.NewFromInt(60),
			Current:      decimal.NewFromInt(70),
			MarketAmount: decimal.NewFromInt(7000),
		},
		{
			Name:         "Debt",
			Target:       decimal.NewFromInt(30),
			Current:      decimal.NewFromInt(20),
			MarketAmount: decimal.NewFromInt(2000),
		},
	}

	plan := ComputeRebalancePlan(targets, decimal.Zero, "full_rebalance")

	assert.Equal(t, "SELL", plan.Items[0].Action)
	assert.True(t, plan.Items[0].Amount.Equal(decimal.NewFromInt(1000)))
	assert.Equal(t, "BUY", plan.Items[1].Action)
	assert.True(t, plan.Items[1].Amount.Equal(decimal.NewFromInt(1000)))

	bought := decimal.Zero
	sold := decimal.Zero
	for _, item := range plan.Items {
		switch item.Action {
		case "BUY":
			bought = bought.Add(item.Amount)
		case "SELL":
			sold = sold.Add(item.Amount)
		}
	}
	assert.True(t, bought.Equal(sold), "buys and sells must net to zero, bought: %s, sold: %s", bought, sold)
}

func TestValidateAllocationTargetsRequiresHundredPercent(t *testing.T) {
	valid := []config.AllocationTarget{
		{Name: "Equity", Target: 60, Drift: 5, Accounts: []string{"Assets:Equity:*"}},
		{Name: "Debt", Target: 40, Drift: 5, Accounts: []string{"Assets:Debt:*"}},
	}
	require.NoError(t, validateAllocationTargets(valid))

	invalid := []config.AllocationTarget{
		{Name: "Equity", Target: 60, Drift: 5, Accounts: []string{"Assets:Equity:*"}},
		{Name: "Debt", Target: 30, Drift: 5, Accounts: []string{"Assets:Debt:*"}},
	}
	require.Error(t, validateAllocationTargets(invalid))
}

func TestRebalanceHandlerRejectsUnknownMode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openFuturePostingsTestDB(t)

	router := gin.New()
	router.POST("/api/allocation/rebalance", RebalanceHandler(db))

	payload, err := json.Marshal(RebalanceRequest{Cash: decimal.NewFromInt(1000), Mode: "sideways"})
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, "/api/allocation/rebalance", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
