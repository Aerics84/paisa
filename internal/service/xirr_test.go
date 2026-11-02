package service

import (
	"strings"
	"testing"
	"time"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/model/cache"
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/ananthakumaran/paisa/internal/model/price"
	"github.com/ananthakumaran/paisa/internal/utils"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openXIRRTestDB(t *testing.T) *gorm.DB {
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

	ClearInterestCache()
	ClearPriceCache()
	t.Cleanup(ClearInterestCache)
	t.Cleanup(ClearPriceCache)

	return db
}

func xirrTestDate(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func dateKey(t time.Time) string {
	return t.Format("2006-01-02")
}

func TestXIRRConvertedUsesDateRates(t *testing.T) {
	db := openXIRRTestDB(t)

	purchaseDate := xirrTestDate(2024, time.January, 1)
	ps := []posting.Posting{
		{
			TransactionID: "buy",
			Date:          purchaseDate,
			Payee:         "Buy",
			Account:       "Assets:Broker",
			Commodity:     "EUR",
			Quantity:      decimal.NewFromInt(10),
			Amount:        decimal.NewFromInt(-1000),
			MarketAmount:  decimal.NewFromInt(1500),
		},
	}

	rates := map[string]decimal.Decimal{
		dateKey(purchaseDate):       decimal.NewFromInt(2),
		dateKey(utils.EndOfToday()): decimal.NewFromInt(4),
	}
	rate := func(date time.Time) decimal.Decimal {
		return rates[dateKey(date)]
	}

	converted := XIRRConverted(db, ps, rate)

	// The purchase cashflow uses the rate on its own date (1000/2), while the
	// closing value uses today's rate (1500/4).
	expected := XIRR(db, []posting.Posting{
		{
			TransactionID: "buy",
			Date:          purchaseDate,
			Payee:         "Buy",
			Account:       "Assets:Broker",
			Commodity:     "EUR",
			Quantity:      decimal.NewFromInt(10),
			Amount:        decimal.NewFromInt(-500),
			MarketAmount:  decimal.NewFromInt(375),
		},
	})

	require.True(t, converted.Equal(expected), "expected %s, got %s", expected, converted)
}

func TestXIRRConvertedSkipsMissingRate(t *testing.T) {
	db := openXIRRTestDB(t)

	firstDate := xirrTestDate(2024, time.January, 1)
	missingDate := xirrTestDate(2024, time.June, 1)
	ps := []posting.Posting{
		{
			TransactionID: "buy",
			Date:          firstDate,
			Payee:         "Buy",
			Account:       "Assets:Broker",
			Commodity:     "EUR",
			Quantity:      decimal.NewFromInt(10),
			Amount:        decimal.NewFromInt(-1000),
			MarketAmount:  decimal.NewFromInt(1200),
		},
		{
			TransactionID: "buy-more",
			Date:          missingDate,
			Payee:         "Buy more",
			Account:       "Assets:Broker",
			Commodity:     "EUR",
			Quantity:      decimal.NewFromInt(1),
			Amount:        decimal.NewFromInt(-100),
		},
	}

	rate := func(date time.Time) decimal.Decimal {
		if dateKey(date) == dateKey(missingDate) {
			return decimal.Zero
		}
		if dateKey(date) == dateKey(utils.EndOfToday()) {
			return decimal.NewFromInt(2)
		}
		return decimal.NewFromInt(2)
	}

	converted := XIRRConverted(db, ps, rate)

	// A missing rate contributes a zero cashflow and must not panic.
	expected := XIRRConverted(db, ps[:1], rate)
	require.True(t, converted.Equal(expected), "expected %s, got %s", expected, converted)
}

func TestXIRRConvertedReturnsZeroWithoutTodayRate(t *testing.T) {
	db := openXIRRTestDB(t)

	purchaseDate := xirrTestDate(2024, time.January, 1)
	ps := []posting.Posting{
		{
			TransactionID: "buy",
			Date:          purchaseDate,
			Payee:         "Buy",
			Account:       "Assets:Broker",
			Commodity:     "EUR",
			Quantity:      decimal.NewFromInt(10),
			Amount:        decimal.NewFromInt(-1000),
			MarketAmount:  decimal.NewFromInt(1200),
		},
	}

	rate := func(date time.Time) decimal.Decimal {
		if dateKey(date) == dateKey(utils.EndOfToday()) {
			return decimal.Zero
		}
		return decimal.NewFromInt(2)
	}

	require.True(t, XIRRConverted(db, ps, rate).Equal(decimal.Zero))
}
