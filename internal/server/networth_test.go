package server

import (
	"strings"
	"testing"
	"time"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/model/cache"
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/ananthakumaran/paisa/internal/model/price"
	"github.com/ananthakumaran/paisa/internal/service"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openNetworthCurrencyTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	require.NoError(t, config.LoadConfig([]byte(`
journal_path: main.ledger
db_path: paisa.db
default_currency: EUR
time_zone: UTC
networth_currencies:
  - name: USD
    symbol: $
    commodity: USD
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

func currencyTestDate(day int) time.Time {
	return time.Date(2024, time.January, day, 0, 0, 0, 0, time.UTC)
}

func TestComputeConvertedNetworthTimeline(t *testing.T) {
	db := openNetworthCurrencyTestDB(t)

	first := currencyTestDate(1)
	second := currencyTestDate(2)
	require.NoError(t, db.Create(&[]price.Price{
		{Date: first, CommodityType: config.Unknown, CommodityID: "USD", CommodityName: "USD", Value: decimal.NewFromInt(2)},
		{Date: second, CommodityType: config.Unknown, CommodityID: "USD", CommodityName: "USD", Value: decimal.NewFromInt(4)},
	}).Error)

	timeline := []Networth{
		{Date: first, InvestmentAmount: decimal.NewFromInt(100), BalanceAmount: decimal.NewFromInt(100)},
		{Date: second, InvestmentAmount: decimal.NewFromInt(200), BalanceAmount: decimal.NewFromInt(220)},
	}

	converted := computeConvertedNetworthTimeline(db, "USD", timeline)
	require.Len(t, converted, 2)

	require.True(t, converted[0].Date.Equal(first))
	require.True(t, converted[0].InvestmentAmount.Equal(decimal.NewFromInt(50)), "got %s", converted[0].InvestmentAmount)
	require.True(t, converted[0].BalanceAmount.Equal(decimal.NewFromInt(50)), "got %s", converted[0].BalanceAmount)
	require.True(t, converted[0].GainAmount.Equal(decimal.Zero), "got %s", converted[0].GainAmount)

	require.True(t, converted[1].Date.Equal(second))
	require.True(t, converted[1].InvestmentAmount.Equal(decimal.NewFromInt(75)), "got %s", converted[1].InvestmentAmount)
	require.True(t, converted[1].BalanceAmount.Equal(decimal.NewFromInt(55)), "got %s", converted[1].BalanceAmount)
	require.True(t, converted[1].GainAmount.Equal(decimal.NewFromInt(-20)), "got %s", converted[1].GainAmount)
	require.True(t, converted[1].NetInvestmentAmount.Equal(decimal.NewFromInt(75)), "got %s", converted[1].NetInvestmentAmount)
}

func TestConvertedTimelineEmptyWithoutRates(t *testing.T) {
	db := openNetworthCurrencyTestDB(t)

	timeline := []Networth{
		{Date: currencyTestDate(1), InvestmentAmount: decimal.NewFromInt(100), BalanceAmount: decimal.NewFromInt(100)},
	}

	converted := computeConvertedNetworthTimeline(db, "USD", timeline)
	require.Empty(t, converted)
}

func TestConvertedTimelineUsesEarliestRateBeforeFirstQuote(t *testing.T) {
	db := openNetworthCurrencyTestDB(t)

	beforeQuote := currencyTestDate(1)
	firstQuote := currencyTestDate(2)
	secondQuote := currencyTestDate(3)
	require.NoError(t, db.Create(&[]price.Price{
		{Date: firstQuote, CommodityType: config.Unknown, CommodityID: "USD", CommodityName: "USD", Value: decimal.NewFromInt(4)},
		{Date: secondQuote, CommodityType: config.Unknown, CommodityID: "USD", CommodityName: "USD", Value: decimal.NewFromInt(5)},
	}).Error)

	timeline := []Networth{
		{Date: beforeQuote, InvestmentAmount: decimal.NewFromInt(100), BalanceAmount: decimal.NewFromInt(100)},
		{Date: firstQuote, InvestmentAmount: decimal.NewFromInt(200), BalanceAmount: decimal.NewFromInt(220)},
		{Date: secondQuote, InvestmentAmount: decimal.NewFromInt(300), BalanceAmount: decimal.NewFromInt(300)},
	}

	converted := computeConvertedNetworthTimeline(db, "USD", timeline)
	require.Len(t, converted, 3)

	require.True(t, converted[0].BalanceAmount.Equal(decimal.NewFromInt(25)), "earliest rate before the first quote must be used, got %s", converted[0].BalanceAmount)
	require.True(t, converted[0].InvestmentAmount.Equal(decimal.NewFromInt(25)), "got %s", converted[0].InvestmentAmount)

	require.True(t, converted[1].BalanceAmount.Equal(decimal.NewFromInt(55)), "got %s", converted[1].BalanceAmount)
	require.True(t, converted[1].InvestmentAmount.Equal(decimal.NewFromInt(50)), "got %s", converted[1].InvestmentAmount)

	require.True(t, converted[2].BalanceAmount.Equal(decimal.NewFromInt(60)), "got %s", converted[2].BalanceAmount)
	require.True(t, converted[2].InvestmentAmount.Equal(decimal.NewFromInt(70)), "got %s", converted[2].InvestmentAmount)
}
