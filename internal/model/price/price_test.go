package price

import (
	"testing"
	"time"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openPriceTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Price{}))

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
	})

	return db
}

func loadPrices(t *testing.T, db *gorm.DB, commodityType config.CommodityType, commodityName string) []Price {
	t.Helper()

	var rows []Price
	require.NoError(t, db.Where("commodity_type = ? and commodity_name = ?", commodityType, commodityName).Order("date").Find(&rows).Error)
	return rows
}

func TestUpsertAllByTypeUnknownKeepsCurrencyQuotes(t *testing.T) {
	db := openPriceTestDB(t)

	date := time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC)
	UpsertAllByTypeNameAndID(db, config.Currency, "USD", "USD", []*Price{
		{
			Date:          date,
			CommodityType: config.Currency,
			CommodityID:   "USD",
			CommodityName: "USD",
			Value:         decimal.NewFromInt(80),
		},
	})

	// Journal-only sync replaces the journal's own quotes without
	// touching quotes fetched for configured commodities.
	UpsertAllByType(db, config.Unknown, []Price{
		{
			Date:          date.AddDate(0, 0, 1),
			CommodityType: config.Unknown,
			CommodityID:   "USD",
			CommodityName: "USD",
			Value:         decimal.NewFromInt(81),
		},
	})

	var rows []Price
	require.NoError(t, db.Order("date").Find(&rows).Error)
	require.Len(t, rows, 2)
	assert.Equal(t, config.Currency, rows[0].CommodityType)
	assert.True(t, rows[0].Value.Equal(decimal.NewFromInt(80)))
	assert.Equal(t, config.Unknown, rows[1].CommodityType)
	assert.True(t, rows[1].Value.Equal(decimal.NewFromInt(81)))
}

func TestUpsertAllByTypeNameAndIDReplacesOnlyCurrencyQuotes(t *testing.T) {
	db := openPriceTestDB(t)

	date := time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC)
	require.NoError(t, db.Create(&Price{
		Date:          date,
		CommodityType: config.Unknown,
		CommodityID:   "USD",
		CommodityName: "USD",
		Value:         decimal.NewFromInt(81),
	}).Error)

	UpsertAllByTypeNameAndID(db, config.Currency, "USD", "USD", []*Price{
		{
			Date:          date,
			CommodityType: config.Currency,
			CommodityID:   "USD",
			CommodityName: "USD",
			Value:         decimal.NewFromInt(80),
		},
	})
	UpsertAllByTypeNameAndID(db, config.Currency, "USD", "USD", []*Price{
		{
			Date:          date.AddDate(0, 0, 1),
			CommodityType: config.Currency,
			CommodityID:   "USD",
			CommodityName: "USD",
			Value:         decimal.NewFromInt(82),
		},
	})

	var currencyRows []Price
	require.NoError(t, db.Where("commodity_type = ?", config.Currency).Find(&currencyRows).Error)
	require.Len(t, currencyRows, 1)
	assert.Equal(t, "2026-10-31", currencyRows[0].Date.Format("2006-01-02"))
	assert.True(t, currencyRows[0].Value.Equal(decimal.NewFromInt(82)))

	var unknownRows []Price
	require.NoError(t, db.Where("commodity_type = ?", config.Unknown).Find(&unknownRows).Error)
	require.Len(t, unknownRows, 1)
	assert.True(t, unknownRows[0].Value.Equal(decimal.NewFromInt(81)))
}

func TestUpsertMergedPreservesRowsMissingFromResponse(t *testing.T) {
	db := openPriceTestDB(t)
	require.NoError(t, UpsertMerged(db, config.Stock, "AAPL", "AAPL", []*Price{
		priceAt(18, "100"), priceAt(19, "101"), priceAt(20, "102"),
	}, true))

	require.NoError(t, UpsertMerged(db, config.Stock, "AAPL", "AAPL", []*Price{
		priceAt(20, "120"), priceAt(21, "121"),
	}, false))

	got := loadPrices(t, db, config.Stock, "AAPL")
	require.Len(t, got, 4)
	assert.Equal(t, "100", got[0].Value.String())
	assert.Equal(t, "101", got[1].Value.String())
	assert.Equal(t, "120", got[2].Value.String())
	assert.Equal(t, "121", got[3].Value.String())
}

func TestUpsertMergedEmptyReplaceDoesNotDelete(t *testing.T) {
	db := openPriceTestDB(t)
	require.NoError(t, UpsertMerged(db, config.Stock, "AAPL", "AAPL", []*Price{priceAt(20, "100")}, true))
	require.Error(t, UpsertMerged(db, config.Stock, "AAPL", "AAPL", nil, true))
	assert.Len(t, loadPrices(t, db, config.Stock, "AAPL"), 1)
}

func TestLatestDate(t *testing.T) {
	db := openPriceTestDB(t)
	require.NoError(t, UpsertMerged(db, config.Stock, "AAPL", "AAPL", []*Price{priceAt(18, "100"), priceAt(20, "102")}, true))
	got, ok, err := LatestDate(db, config.Stock, "AAPL", "AAPL")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC), got)
}

func TestUpsertMergedHandlesLargeSeries(t *testing.T) {
	db := openPriceTestDB(t)

	const count = 7001
	start := time.Date(2010, 1, 1, 0, 0, 0, 0, time.UTC)
	prices := make([]*Price, 0, count)
	for i := 0; i < count; i++ {
		prices = append(prices, &Price{
			Date:  start.AddDate(0, 0, i),
			Value: decimal.NewFromInt(int64(i + 1)),
		})
	}

	require.NoError(t, UpsertMerged(db, config.Stock, "AAPL", "AAPL", prices, true))

	got := loadPrices(t, db, config.Stock, "AAPL")
	require.Len(t, got, count)
	assert.Equal(t, "1", got[0].Value.String())
	assert.Equal(t, "7001", got[count-1].Value.String())
}
