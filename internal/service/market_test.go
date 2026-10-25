package service

import (
	"os"
	"os/exec"
	"sync"
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

func TestGetUnitPriceReturnsLastQuoteOnOrBeforeDate(t *testing.T) {
	loadMarketTestConfig(t)
	ClearPriceCache()

	db := openTestDB(t)
	firstQuote := time.Date(2024, 3, 1, 0, 0, 0, 0, config.TimeZone())
	secondQuote := time.Date(2024, 3, 5, 0, 0, 0, 0, config.TimeZone())

	require.NoError(t, db.Create(&price.Price{
		Date:          firstQuote,
		CommodityType: config.Stock,
		CommodityID:   "asof-1",
		CommodityName: "ASOF",
		Value:         decimal.NewFromInt(100),
	}).Error)
	require.NoError(t, db.Create(&price.Price{
		Date:          secondQuote,
		CommodityType: config.Stock,
		CommodityID:   "asof-1",
		CommodityName: "ASOF",
		Value:         decimal.NewFromInt(110),
	}).Error)

	before, ok := FindUnitPrice(db, "ASOF", time.Date(2024, 2, 29, 0, 0, 0, 0, config.TimeZone()))
	require.False(t, ok, "a lookup before the first quote must not invent a price")
	assert.True(t, before.Value.Equal(decimal.Zero))

	onFirst, ok := FindUnitPrice(db, "ASOF", firstQuote)
	require.True(t, ok)
	assert.True(t, onFirst.Date.Equal(firstQuote))
	assert.True(t, onFirst.Value.Equal(decimal.NewFromInt(100)))

	between, ok := FindUnitPrice(db, "ASOF", time.Date(2024, 3, 3, 0, 0, 0, 0, config.TimeZone()))
	require.True(t, ok)
	assert.True(t, between.Date.Equal(firstQuote), "the quote date must identify the carried-forward price")
	assert.True(t, between.Value.Equal(decimal.NewFromInt(100)))

	onSecond, ok := FindUnitPrice(db, "ASOF", secondQuote)
	require.True(t, ok)
	assert.True(t, onSecond.Date.Equal(secondQuote))
	assert.True(t, onSecond.Value.Equal(decimal.NewFromInt(110)))

	_, ok = FindUnitPrice(db, "NO_SUCH_COMMODITY", secondQuote)
	require.False(t, ok, "an unknown commodity must be reported as unavailable, not fatal")
}

func TestGetUnitPriceCarriesQuoteThroughWeekend(t *testing.T) {
	loadMarketTestConfig(t)
	ClearPriceCache()

	db := openTestDB(t)
	friday := time.Date(2024, 3, 8, 0, 0, 0, 0, config.TimeZone())
	sunday := time.Date(2024, 3, 10, 0, 0, 0, 0, config.TimeZone())

	require.NoError(t, db.Create(&price.Price{
		Date:          friday,
		CommodityType: config.Stock,
		CommodityID:   "weekend-1",
		CommodityName: "WEEKEND",
		Value:         decimal.NewFromInt(50),
	}).Error)

	quote, ok := FindUnitPrice(db, "WEEKEND", sunday)
	require.True(t, ok)
	assert.True(t, quote.Date.Equal(friday), "weekends carry the Friday quote and expose its actual date")
	assert.True(t, quote.Value.Equal(decimal.NewFromInt(50)))
}

func TestGetUnitPriceReturnsEmptyPriceForUnknownCommodity(t *testing.T) {
	loadMarketTestConfig(t)
	ClearPriceCache()

	db := openTestDB(t)
	quote := GetUnitPrice(db, "NO_GET_PRICE", time.Date(2024, 1, 2, 0, 0, 0, 0, config.TimeZone()))

	assert.True(t, quote.Value.Equal(decimal.Zero), "an unknown commodity must fall back to a zero quote instead of terminating")
}

func TestGetUnitPriceReturnsZeroWithoutPanic(t *testing.T) {
	if os.Getenv("PAISA_TEST_MISSING_PRICE") == "1" {
		quote := GetUnitPrice(openTestDB(t), "MISSING", time.Now())
		if !quote.Value.IsZero() {
			os.Exit(2)
		}
		os.Exit(0)
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestGetUnitPriceReturnsZeroWithoutPanic")
	cmd.Env = append(os.Environ(), "PAISA_TEST_MISSING_PRICE=1")
	require.NoError(t, cmd.Run())
}

func TestPriceCacheConcurrentClearAndRead(t *testing.T) {
	db := openTestDB(t)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); _, _ = FindUnitPrice(db, "EUR", time.Now()) }()
		go func() { defer wg.Done(); ClearPriceCache() }()
	}
	wg.Wait()
}
