package service

import (
	"testing"
	"time"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openContributionTestDB(t *testing.T, name string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&posting.Posting{}))
	return db
}

func contributionTestPosting(date string, transactionID, account, commodity string, amount, quantity string) posting.Posting {
	d, err := time.ParseInLocation("2006-01-02", date, time.UTC)
	if err != nil {
		panic(err)
	}
	return posting.Posting{
		Date:          d,
		TransactionID: transactionID,
		Account:       account,
		Commodity:     commodity,
		Amount:        decimal.RequireFromString(amount),
		Quantity:      decimal.RequireFromString(quantity),
	}
}

func TestMonthlyContributionsMatchesParentAccount(t *testing.T) {
	asOf := time.Date(2026, time.September, 5, 0, 0, 0, 0, time.UTC)
	currency := config.DefaultCurrency()
	posts := []posting.Posting{
		contributionTestPosting("2026-06-01", "saving", "Assets:Savings:Bank", currency, "10000", "10000"),
		contributionTestPosting("2026-06-01", "saving", "Assets:Outside", currency, "-10000", "-10000"),
	}
	db := openContributionTestDB(t, "contributions-parent-account")
	require.NoError(t, db.Create(&posts).Error)

	result := MonthlyContributions(db, posts, []string{"Assets:Savings"}, asOf)

	require.Len(t, result, 3)
	require.True(t, decimal.RequireFromString("10000").Equal(result[0].Amount), "expected 10000 got %s", result[0].Amount)
	require.True(t, result[1].Amount.IsZero())
	require.True(t, result[2].Amount.IsZero())
}

func TestMonthlyContributionsClassifiesCashFlows(t *testing.T) {
	asOf := time.Date(2026, time.September, 5, 0, 0, 0, 0, time.UTC)
	currency := config.DefaultCurrency()
	tests := []struct {
		name    string
		posts   []posting.Posting
		amounts []string
	}{
		{
			name: "interest credited to a savings asset is not a contribution",
			posts: []posting.Posting{
				contributionTestPosting("2026-06-01", "interest", "Assets:Goal:Bank", currency, "500", "500"),
				contributionTestPosting("2026-06-01", "interest", "Income:Interest:Goal:Bank", currency, "-500", "-500"),
			},
			amounts: []string{"0", "0", "0"},
		},
		{
			name: "an internal sale within goal accounts nets to zero",
			posts: []posting.Posting{
				contributionTestPosting("2026-06-01", "buy", "Assets:Goal:Fund", "GOAL", "10000", "10"),
				contributionTestPosting("2026-06-01", "buy", "Assets:Outside", currency, "-10000", "-10000"),
				contributionTestPosting("2026-07-01", "sale", "Assets:Goal:Fund", "GOAL", "-10000", "-10"),
				contributionTestPosting("2026-07-01", "sale", "Assets:Goal:Bank", currency, "15000", "15000"),
				contributionTestPosting("2026-07-01", "sale", "Income:CapitalGains:Goal:Fund", currency, "-5000", "-5000"),
			},
			amounts: []string{"10000", "0", "0"},
		},
		{
			name: "a normal contribution is counted",
			posts: []posting.Posting{
				contributionTestPosting("2026-06-01", "saving", "Assets:Goal:Bank", currency, "10000", "10000"),
				contributionTestPosting("2026-06-01", "saving", "Assets:Outside", currency, "-10000", "-10000"),
			},
			amounts: []string{"10000", "0", "0"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := openContributionTestDB(t, "contributions-"+tt.name)
			require.NoError(t, db.Create(&tt.posts).Error)

			result := MonthlyContributions(db, tt.posts, []string{"Assets:Goal:*"}, asOf)

			require.Len(t, result, len(tt.amounts))
			for i, amount := range tt.amounts {
				require.True(t, decimal.RequireFromString(amount).Equal(result[i].Amount), "month %s: expected %s got %s", result[i].Month, amount, result[i].Amount)
			}
			require.Equal(t, "2026-06", result[0].Month)
		})
	}
}
