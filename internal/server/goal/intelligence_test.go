package goal

import (
	"testing"
	"time"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func contributionTestDB(t *testing.T, name string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&posting.Posting{}))
	return db
}

func contributionPosting(date time.Time, transactionID, account string, amount int64) posting.Posting {
	return posting.Posting{
		Date:          date,
		TransactionID: transactionID,
		Account:       account,
		Commodity:     config.DefaultCurrency(),
		Amount:        decimal.NewFromInt(amount),
		Quantity:      decimal.NewFromInt(amount),
	}
}

func TestContributionHistoryNetsAnInternalSale(t *testing.T) {
	asOf := time.Date(2026, time.September, 5, 0, 0, 0, 0, time.UTC)
	currency := config.DefaultCurrency()
	postings := []posting.Posting{
		{Date: time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC), TransactionID: "buy", Account: "Assets:Goal:Fund", Commodity: "GOAL", Amount: decimal.NewFromInt(10000), Quantity: decimal.NewFromInt(10)},
		contributionPosting(time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC), "buy", "Assets:Outside", -10000),
		{Date: time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC), TransactionID: "sale", Account: "Assets:Goal:Fund", Commodity: "GOAL", Amount: decimal.NewFromInt(-10000), Quantity: decimal.NewFromInt(-10)},
		contributionPosting(time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC), "sale", "Assets:Goal:Bank", 15000),
		{Date: time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC), TransactionID: "sale", Account: "Income:CapitalGains:Goal:Fund", Commodity: currency, Amount: decimal.NewFromInt(-5000), Quantity: decimal.NewFromInt(-5000)},
	}
	db := contributionTestDB(t, "contribution-history-internal-sale")
	require.NoError(t, db.Create(&postings).Error)

	history := contributionHistory(db, postings, []string{"Assets:Goal:*"}, asOf)

	require.Len(t, history, 3)
	assert.Equal(t, 10000.0, history[0].Amount)
	assert.Equal(t, 0.0, history[1].Amount)
	assert.Equal(t, 0.0, history[2].Amount)
}

func TestContributionHistoryIgnoresInterestIncome(t *testing.T) {
	asOf := time.Date(2026, time.September, 5, 0, 0, 0, 0, time.UTC)
	postings := []posting.Posting{
		contributionPosting(time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC), "interest", "Assets:Goal:Bank", 500),
		contributionPosting(time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC), "interest", "Income:Interest:Goal:Bank", -500),
	}
	db := contributionTestDB(t, "contribution-history-interest")
	require.NoError(t, db.Create(&postings).Error)

	history := contributionHistory(db, postings, []string{"Assets:Goal:*"}, asOf)

	require.Len(t, history, 3)
	for _, month := range history {
		assert.Equal(t, 0.0, month.Amount, month.Month)
	}
}

func TestContributionHistoryKeepsZeroMonthsAndExcludesCurrentMonth(t *testing.T) {
	asOf := time.Date(2026, time.September, 5, 0, 0, 0, 0, time.UTC)
	postings := []posting.Posting{
		contributionPosting(time.Date(2026, time.June, 10, 0, 0, 0, 0, time.UTC), "june", "Assets:Goal:Bank", 10000),
		contributionPosting(time.Date(2026, time.August, 10, 0, 0, 0, 0, time.UTC), "august", "Assets:Goal:Bank", 10000),
		contributionPosting(time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC), "september", "Assets:Goal:Bank", 900000),
		contributionPosting(time.Date(2026, time.June, 10, 0, 0, 0, 0, time.UTC), "other", "Assets:Other", 50000),
	}
	db := contributionTestDB(t, "contribution-history-zero-months")
	require.NoError(t, db.Create(&postings).Error)

	history := contributionHistory(db, postings, []string{"Assets:Goal:*"}, asOf)

	require.Len(t, history, 3)
	assert.Equal(t, "2026-06", history[0].Month)
	assert.Equal(t, 10000.0, history[0].Amount)
	assert.Equal(t, "2026-07", history[1].Month)
	assert.Equal(t, 0.0, history[1].Amount)
	assert.Equal(t, "2026-08", history[2].Month)
	assert.Equal(t, 10000.0, history[2].Amount)
}

func TestContributionHistoryStopsAtSixCompleteMonths(t *testing.T) {
	asOf := time.Date(2026, time.September, 5, 0, 0, 0, 0, time.UTC)
	postings := []posting.Posting{
		contributionPosting(time.Date(2025, time.January, 10, 0, 0, 0, 0, time.UTC), "old", "Assets:Goal:Bank", 1000),
	}
	for _, month := range []time.Month{time.March, time.April, time.May, time.June, time.July, time.August} {
		postings = append(postings, contributionPosting(
			time.Date(2026, month, 10, 0, 0, 0, 0, time.UTC),
			month.String(),
			"Assets:Goal:Bank",
			2000,
		))
	}
	db := contributionTestDB(t, "contribution-history-six-months")
	require.NoError(t, db.Create(&postings).Error)

	history := contributionHistory(db, postings, []string{"Assets:Goal:*"}, asOf)

	require.Len(t, history, 6)
	assert.Equal(t, "2026-03", history[0].Month)
	assert.Equal(t, "2026-08", history[5].Month)
}
