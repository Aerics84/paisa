package goal

import (
	"testing"
	"time"

	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContributionHistoryKeepsZeroMonthsAndExcludesCurrentMonth(t *testing.T) {
	asOf := time.Date(2026, time.September, 5, 0, 0, 0, 0, time.UTC)
	postings := []posting.Posting{
		{Date: time.Date(2026, time.June, 10, 0, 0, 0, 0, time.UTC), Account: "Assets:Goal:Bank", Amount: decimal.NewFromInt(10000)},
		{Date: time.Date(2026, time.August, 10, 0, 0, 0, 0, time.UTC), Account: "Assets:Goal:Bank", Amount: decimal.NewFromInt(10000)},
		{Date: time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC), Account: "Assets:Goal:Bank", Amount: decimal.NewFromInt(900000)},
		{Date: time.Date(2026, time.June, 10, 0, 0, 0, 0, time.UTC), Account: "Assets:Other", Amount: decimal.NewFromInt(50000)},
	}

	history := contributionHistory(postings, []string{"Assets:Goal:*"}, asOf)

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
		{Date: time.Date(2025, time.January, 10, 0, 0, 0, 0, time.UTC), Account: "Assets:Goal:Bank", Amount: decimal.NewFromInt(1000)},
	}
	for _, month := range []time.Month{time.March, time.April, time.May, time.June, time.July, time.August} {
		postings = append(postings, posting.Posting{
			Date:    time.Date(2026, month, 10, 0, 0, 0, 0, time.UTC),
			Account: "Assets:Goal:Bank",
			Amount:  decimal.NewFromInt(2000),
		})
	}

	history := contributionHistory(postings, []string{"Assets:Goal:*"}, asOf)

	require.Len(t, history, 6)
	assert.Equal(t, "2026-03", history[0].Month)
	assert.Equal(t, "2026-08", history[5].Month)
}
