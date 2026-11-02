package server

import (
	"testing"

	"github.com/ananthakumaran/paisa/internal/accounting"
	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/model/cache"
	"github.com/ananthakumaran/paisa/internal/model/portfolio"
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/ananthakumaran/paisa/internal/model/price"
	"github.com/ananthakumaran/paisa/internal/server/assets"
	"github.com/ananthakumaran/paisa/internal/service"
	"github.com/ananthakumaran/paisa/internal/utils"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestGainExcludesFuturePostings(t *testing.T) {
	db := openFuturePostingsTestDB(t)

	today, future := futurePostingsTestDates()
	addFuturePostingTestPosting(t, db, "today", today, "Assets:Broker", 100)
	addFuturePostingTestPosting(t, db, "future", future, "Assets:Broker", 50)

	result := GetGain(db)
	gains, ok := result["gain_breakdown"].([]Gain)
	require.True(t, ok)
	require.Len(t, gains, 1)

	require.True(t, gains[0].Networth.BalanceAmount.Equal(decimal.NewFromInt(100)), "future postings must not inflate the gain balance, got %s", gains[0].Networth.BalanceAmount)
}

func TestAccountGainExcludesFuturePostings(t *testing.T) {
	require.NoError(t, config.LoadConfig([]byte(`
journal_path: main.ledger
db_path: paisa.db
default_currency: EUR
time_zone: UTC
commodities:
  - name: STOCKX
    type: stock
    price:
      provider: com-yahoo
      code: STOCKX
`), ""))

	db, err := gorm.Open(sqlite.Open("file:account-gain-future?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&posting.Posting{}, &cache.Cache{}, &price.Price{}, &portfolio.Portfolio{}))

	service.ClearInterestCache()
	service.ClearPriceCache()
	accounting.ClearCache()
	t.Cleanup(service.ClearInterestCache)
	t.Cleanup(service.ClearPriceCache)
	t.Cleanup(accounting.ClearCache)

	require.NoError(t, db.Create(&portfolio.Portfolio{
		ParentCommodityID: "STOCKX",
		SecurityName:      "Stock X",
		SecurityType:      "equity",
		Percentage:        decimal.NewFromInt(100),
	}).Error)

	today, future := futurePostingsTestDates()
	rows := []posting.Posting{
		{TransactionID: "today", Date: today, Payee: "Buy", Account: "Assets:Broker", Commodity: "STOCKX", Quantity: decimal.NewFromInt(1), Amount: decimal.NewFromInt(100)},
		{TransactionID: "future", Date: future, Payee: "Buy", Account: "Assets:Broker", Commodity: "STOCKX", Quantity: decimal.NewFromInt(1), Amount: decimal.NewFromInt(50)},
	}
	require.NoError(t, db.Create(&rows).Error)

	result := GetAccountGain(db, "Assets:Broker")

	breakdown, ok := result["asset_breakdown"].(assets.AssetBreakdown)
	require.True(t, ok)
	require.True(t, breakdown.MarketAmount.Equal(decimal.NewFromInt(100)), "future postings must not inflate the asset breakdown, got %s", breakdown.MarketAmount)

	allocation, ok := result["portfolio_allocation"].(PortfolioAllocationGroups)
	require.True(t, ok)
	require.Contains(t, allocation.Commomdities, "STOCKX", "the allocation panel must not be blanked")
	require.NotEmpty(t, allocation.NameAndSecurityType, "the allocation panel must not be blanked")

	total := decimal.Zero
	for _, aggregate := range allocation.NameAndSecurityType {
		total = total.Add(aggregate.Amount)
	}
	require.True(t, total.Equal(decimal.NewFromInt(100)), "future postings must not inflate the portfolio allocation, got %s", total)
}

func TestAccountGainPostingReturnsReferencePostings(t *testing.T) {
	require.NoError(t, config.LoadConfig([]byte(`
journal_path: main.ledger
db_path: paisa.db
default_currency: EUR
time_zone: UTC
commodities:
  - name: STOCKX
    type: stock
    price:
      provider: com-yahoo
      code: STOCKX
`), ""))

	db, err := gorm.Open(sqlite.Open("file:account-gain-posting-returns?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&posting.Posting{}, &cache.Cache{}, &price.Price{}, &portfolio.Portfolio{}))

	service.ClearInterestCache()
	service.ClearPriceCache()
	accounting.ClearCache()
	t.Cleanup(service.ClearInterestCache)
	t.Cleanup(service.ClearPriceCache)
	t.Cleanup(accounting.ClearCache)

	require.NoError(t, db.Create(&portfolio.Portfolio{
		ParentCommodityID: "STOCKX",
		SecurityName:      "Stock X",
		SecurityType:      "equity",
		Percentage:        decimal.NewFromInt(100),
	}).Error)

	today := utils.Now()
	rows := []posting.Posting{
		// Applicable lot: positive units and cost, held for two years.
		{TransactionID: "lot", Date: today.AddDate(-2, 0, 0), Payee: "Buy", Account: "Assets:Broker", Commodity: "STOCKX", Quantity: decimal.NewFromInt(1), Amount: decimal.NewFromInt(100)},
		// Not applicable: bought today, a sale, and a currency posting.
		{TransactionID: "today", Date: today, Payee: "Buy", Account: "Assets:Broker", Commodity: "STOCKX", Quantity: decimal.NewFromInt(1), Amount: decimal.NewFromInt(150)},
		{TransactionID: "sale", Date: today.AddDate(-1, 0, 0), Payee: "Sell", Account: "Assets:Broker", Commodity: "STOCKX", Quantity: decimal.NewFromInt(-1), Amount: decimal.NewFromInt(-120)},
		{TransactionID: "cash", Date: today.AddDate(-1, 0, 0), Payee: "Deposit", Account: "Assets:Broker", Commodity: "EUR", Quantity: decimal.NewFromInt(50), Amount: decimal.NewFromInt(50)},
	}
	require.NoError(t, db.Create(&rows).Error)

	require.NoError(t, db.Create(&price.Price{
		Date:          today,
		CommodityType: config.Stock,
		CommodityID:   "STOCKX",
		CommodityName: "STOCKX",
		Value:         decimal.NewFromInt(150),
	}).Error)

	result := GetAccountGain(db, "Assets:Broker")

	gain, ok := result["gain_timeline_breakdown"].(AccountGain)
	require.True(t, ok)
	require.Len(t, gain.PostingReturns, 1, "only the open lot should produce a return")

	postingIDs := make(map[uint]bool, len(gain.Postings))
	for _, p := range gain.Postings {
		postingIDs[p.ID] = true
	}

	lot := gain.PostingReturns[0]
	require.Equal(t, rows[0].ID, lot.PostingID, "the return must reference its source posting")
	require.True(t, postingIDs[lot.PostingID], "the referenced posting must still be present in postings")

	for _, p := range gain.Postings {
		switch p.TransactionID {
		case "today", "sale", "cash":
			require.NotEqual(t, p.ID, lot.PostingID, "%s must not produce a posting return", p.TransactionID)
		}
	}
}
