package accounting

import (
	"testing"
	"time"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/ananthakumaran/paisa/internal/utils"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunningBalanceStartsAtConfiguredMidnight(t *testing.T) {	require.NoError(t, config.LoadConfig([]byte(
		"journal_path: test.ledger\ndb_path: test.db\ntime_zone: Europe/Berlin\ndefault_currency: EUR\n",
	), ""))
	utils.SetNow("2026-06-10")

	date := time.Date(2026, 6, 6, 0, 0, 0, 0, time.UTC)
	series := RunningBalance(nil, []posting.Posting{{
		Date: date, Commodity: "EUR", Quantity: decimal.NewFromInt(10),
	}})

	require.NotEmpty(t, series)
	assert.Equal(t, utils.BeginningOfDay(date), series[0].Date)
}

func TestFilterByGlobCombinesPositiveAndNegativePatterns(t *testing.T) {
	ps := []posting.Posting{
		{Account: "Assets:Bank:Checking"},
		{Account: "Assets:Broker"},
		{Account: "Assets:Cash"},
		{Account: "Expenses:Food"},
	}

	accounts := func(in []posting.Posting) []string {
		out := make([]string, len(in))
		for i, p := range in {
			out[i] = p.Account
		}
		return out
	}

	assert.Equal(t, []string{"Assets:Bank:Checking", "Assets:Broker"},
		accounts(FilterByGlob(ps, []string{"Assets:*", "!Assets:Cash"})))
	assert.Equal(t, []string{"Assets:Bank:Checking", "Assets:Broker"},
		accounts(FilterByGlob(ps, []string{"Assets:Bank", "Assets:Broker"})))
	assert.Equal(t, []string{"Assets:Broker", "Assets:Cash", "Expenses:Food"},
		accounts(FilterByGlob(ps, []string{"!Assets:Bank"})))
	assert.Equal(t, []string{"Assets:Bank:Checking", "Assets:Broker", "Assets:Cash", "Expenses:Food"},
		accounts(FilterByGlob(ps, nil)))
}
