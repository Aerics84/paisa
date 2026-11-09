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
	"github.com/ananthakumaran/paisa/internal/utils"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openBudgetTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	require.NoError(t, config.LoadConfig([]byte(`
journal_path: main.ledger
db_path: paisa.db
default_currency: EUR
time_zone: UTC
budget:
  rollover: yes
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

func budgetTestDate(t *testing.T, date string) time.Time {
	t.Helper()

	parsed, err := time.ParseInLocation("2006-01-02", date, config.TimeZone())
	require.NoError(t, err, "test date %s must be valid", date)
	return parsed
}

func budgetTestPosting(t *testing.T, date string, account string, amount int64) posting.Posting {
	t.Helper()

	return posting.Posting{
		Date:      budgetTestDate(t, date),
		Account:   account,
		Commodity: "EUR",
		Quantity:  decimal.NewFromInt(amount),
		Amount:    decimal.NewFromInt(amount),
	}
}

func TestComputeBudgetDoesNotSubtractRolloverTwice(t *testing.T) {
	db := openBudgetTestDB(t)
	utils.SetNow("2022-02-15")
	t.Cleanup(func() { utils.SetNow(time.Now().Format("2006-01-02")) })

	require.NoError(t, db.Create(&posting.Posting{
		Date:      budgetTestDate(t, "2022-01-01"),
		Account:   "Assets:Checking:Bank",
		Commodity: "EUR",
		Quantity:  decimal.NewFromInt(1000),
		Amount:    decimal.NewFromInt(1000),
	}).Error)

	forecasts := []posting.Posting{
		budgetTestPosting(t, "2022-01-01", "Expenses:Food", 100),
		budgetTestPosting(t, "2022-02-01", "Expenses:Food", 100),
	}

	result := computeBudet(db, forecasts, nil)
	budgetsByMonth, ok := result["budgetsByMonth"].(map[string]Budget)
	require.True(t, ok)

	january := budgetsByMonth["2022-01"]
	require.True(t, january.EndOfMonthBalance.Equal(decimal.NewFromInt(900)), "january end of month balance should reserve the first budget, got %s", january.EndOfMonthBalance)

	february := budgetsByMonth["2022-02"]
	require.True(t, february.EndOfMonthBalance.Equal(decimal.NewFromInt(800)), "february should only reserve its new budget, not the rollover again, got %s", february.EndOfMonthBalance)
}
