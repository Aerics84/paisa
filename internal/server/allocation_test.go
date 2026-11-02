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
