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
