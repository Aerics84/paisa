package server

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/ananthakumaran/paisa/internal/model/price"
	"github.com/ananthakumaran/paisa/internal/model/transaction"
	"github.com/ananthakumaran/paisa/internal/service"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openDoctorTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	err := config.LoadConfig([]byte(`
journal_path: main.ledger
db_path: paisa.db
default_currency: EUR
`), "")
	require.NoError(t, err)

	name := strings.NewReplacer("/", "_", "\\", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&posting.Posting{}, &price.Price{}))

	service.ClearPriceCache()
	transaction.ClearCache()
	t.Cleanup(func() {
		service.ClearPriceCache()
		transaction.ClearCache()
	})

	return db
}

func addDoctorPrice(t *testing.T, db *gorm.DB, commodity string, date string, value string) {
	t.Helper()

	parsed, err := time.Parse("2006-01-02", date)
	require.NoError(t, err)

	require.NoError(t, db.Create(&price.Price{
		Date:          parsed,
		CommodityType: config.Stock,
		CommodityID:   commodity,
		CommodityName: commodity,
		Value:         decimal.RequireFromString(value),
	}).Error)
	service.ClearPriceCache()
}

func addDoctorPosting(t *testing.T, db *gorm.DB, id string, account string, commodity string, date string, quantity string, amount string) {
	t.Helper()

	parsed, err := time.Parse("2006-01-02", date)
	require.NoError(t, err)

	require.NoError(t, db.Create(&posting.Posting{
		TransactionID:        id,
		Date:                 parsed,
		Account:              account,
		Commodity:            commodity,
		Quantity:             decimal.RequireFromString(quantity),
		Amount:               decimal.RequireFromString(amount),
		FileName:             "main.ledger",
		TransactionBeginLine: 1,
	}).Error)
}

func TestDoctorToleratesExecutionPriceNearDailyClose(t *testing.T) {
	db := openDoctorTestDB(t)

	addDoctorPrice(t, db, "IE00B4L5Y983", "2026-10-01", "130")
	addDoctorPosting(t, db, "buy-1", "Assets:Equity:IE00B4L5Y983", "IE00B4L5Y983", "2026-10-01", "2", "262")

	assert.Empty(t, ruleJournalPriceMismatch(db))
}

func TestDoctorFlagsPriceFarFromDailyClose(t *testing.T) {
	db := openDoctorTestDB(t)

	addDoctorPrice(t, db, "GB00B3MBS747", "2026-10-01", "3.88")
	addDoctorPosting(t, db, "buy-1", "Assets:Equity:GB00B3MBS747", "GB00B3MBS747", "2026-10-01", "1", "392")

	issues := ruleJournalPriceMismatch(db)
	require.Len(t, issues, 1)
	assert.Contains(t, issues[0].Error(), "GB00B3MBS747")
}

func TestDoctorSkipsCorporateActionAdjustedPriceSeries(t *testing.T) {
	db := openDoctorTestDB(t)

	// Yahoo restates historical closes after a split, so every posting before
	// the split is off by the split factor while the journal keeps the price
	// that was actually paid.
	for i, date := range []string{"2021-01-04", "2021-02-12", "2021-03-17"} {
		addDoctorPrice(t, db, "US0231351067", date, "130")
		addDoctorPosting(t, db, fmt.Sprintf("buy-%d", i), "Assets:Equity:US0231351067", "US0231351067", date, "0.01", "26")
	}

	assert.Empty(t, ruleJournalPriceMismatch(db))
}

func TestDoctorStillFlagsSingleTypoInAdjustedSeries(t *testing.T) {
	db := openDoctorTestDB(t)

	addDoctorPrice(t, db, "DE0008232125", "2026-10-01", "7.5")
	addDoctorPosting(t, db, "buy-1", "Assets:Equity:DE0008232125", "DE0008232125", "2026-10-01", "10", "75")
	addDoctorPosting(t, db, "buy-2", "Assets:Equity:DE0008232125", "DE0008232125", "2026-10-01", "10", "90")

	issues := ruleJournalPriceMismatch(db)
	require.Len(t, issues, 1)
	assert.Contains(t, issues[0].Error(), "10.0000 DE0008232125 @ 9.0000")
}

func TestDoctorReportsNegativeExpenseEntriesAsSingleHint(t *testing.T) {
	db := openDoctorTestDB(t)

	addDoctorPosting(t, db, "refund-1", "Expenses:Health", "EUR", "2026-05-14", "-200", "-200")
	addDoctorPosting(t, db, "refund-2", "Expenses:Health", "EUR", "2026-06-14", "-80", "-80")
	addDoctorPosting(t, db, "refund-3", "Expenses:Shopping:Clothing", "EUR", "2026-07-14", "-125", "-125")
	addDoctorPosting(t, db, "expense-1", "Expenses:Shopping:Clothing", "EUR", "2026-07-15", "40", "40")

	issues := ruleNonDebitAccount(db)
	require.Len(t, issues, 1)
	assert.Contains(t, issues[0].Error(), "3 postings with a debit entry, -405.00 EUR in total")
	assert.Contains(t, issues[0].Error(), "Expenses:Health</b>: 2 entries")
	assert.Contains(t, issues[0].Error(), "Expenses:Shopping:Clothing</b>: 1 entry")
}
