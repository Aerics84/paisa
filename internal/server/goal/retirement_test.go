package goal

import (
	"testing"
	"time"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/model/cache"
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/ananthakumaran/paisa/internal/model/price"
	"github.com/ananthakumaran/paisa/internal/service"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRetirementDetailExcludesInterestFromInvestmentTotal(t *testing.T) {
	require.NoError(t, config.LoadConfig([]byte(`
journal_path: main.ledger
db_path: paisa.db
default_currency: EUR
time_zone: UTC
`), ""))

	db, err := gorm.Open(sqlite.Open("file:retirement-interest?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&posting.Posting{}, &cache.Cache{}, &price.Price{}))

	service.ClearInterestCache()
	service.ClearPriceCache()
	t.Cleanup(service.ClearInterestCache)
	t.Cleanup(service.ClearPriceCache)

	date := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	rows := []posting.Posting{
		{TransactionID: "buy", Date: date, Payee: "Buy", Account: "Assets:Test", Commodity: "EUR", Quantity: decimal.NewFromInt(100), Amount: decimal.NewFromInt(100)},
		{TransactionID: "interest", Date: date, Payee: "Interest", Account: "Assets:Test", Commodity: "EUR", Quantity: decimal.NewFromInt(2), Amount: decimal.NewFromInt(2)},
		{TransactionID: "interest", Date: date, Payee: "Interest", Account: "Income:Interest:Test", Commodity: "EUR", Quantity: decimal.NewFromInt(-2), Amount: decimal.NewFromInt(-2)},
	}
	require.NoError(t, db.Create(&rows).Error)

	detail := getRetirementDetail(db, config.RetirementGoal{
		Name:           "Test",
		Savings:        []string{"Assets:Test"},
		YearlyExpenses: 45000,
		SWR:            4,
	})

	investmentTotal, ok := detail["investmentTotal"].(decimal.Decimal)
	require.True(t, ok)
	gainTotal, ok := detail["gainTotal"].(decimal.Decimal)
	require.True(t, ok)

	assert.True(t, investmentTotal.Equal(decimal.NewFromInt(100)), "interest must not inflate the investment basis, got %s", investmentTotal)
	assert.True(t, gainTotal.Equal(decimal.NewFromInt(2)), "interest must show up as gain, got %s", gainTotal)
}
