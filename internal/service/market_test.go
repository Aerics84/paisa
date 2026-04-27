package service

import (
	"testing"
	"time"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/ananthakumaran/paisa/internal/model/price"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func loadMarketTestConfig(t *testing.T) {
	t.Helper()

	err := config.LoadConfig([]byte(`
journal_path: main.ledger
db_path: paisa.db
default_currency: EUR
time_zone: Europe/Berlin
`), "")
	require.NoError(t, err)
}

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&price.Price{}, &posting.Posting{}))
	return db
}

func TestMetalValuationUsesStoredUnitPrice(t *testing.T) {
	loadMarketTestConfig(t)
	ClearPriceCache()

	db := openTestDB(t)
	valuationDate := time.Date(2024, 1, 2, 0, 0, 0, 0, config.TimeZone())

	require.NoError(t, db.Create(&price.Price{
		Date:          valuationDate,
		CommodityType: config.Metal,
		CommodityID:   "gold-999",
		CommodityName: "GOLD",
		Value:         decimal.NewFromInt(100),
	}).Error)

	require.NoError(t, db.Create(&posting.Posting{
		Date:      valuationDate,
		Account:   "Assets:Gold",
		Commodity: "GOLD",
		Quantity:  decimal.NewFromInt(2),
		Amount:    decimal.NewFromInt(160),
	}).Error)

	assert.True(t, GetPrice(db, "GOLD", decimal.NewFromInt(2), valuationDate).Equal(decimal.NewFromInt(200)))

	postingValue := GetMarketPrice(db, posting.Posting{
		Date:      valuationDate,
		Account:   "Assets:Gold",
		Commodity: "GOLD",
		Quantity:  decimal.NewFromInt(2),
		Amount:    decimal.NewFromInt(160),
	}, valuationDate)
	assert.True(t, postingValue.Equal(decimal.NewFromInt(200)))
}
