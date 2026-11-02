package server

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestGainExcludesFuturePostings(t *testing.T) {
	db := openFuturePostingsTestDB(t)

	today, future := futurePostingsTestDates()
	addFuturePostingTestPosting(t, db, "today", today, "Assets:Broker", 100)
	addFuturePostingTestPosting(t, db, "future", future, "Assets:Broker", 50)

	result := GetGain(db)
	gains, ok := result["gain_breakdown"].([]Gain)
	require.True(t, ok)
	require.Len(t, gains, 1)

	require.True(t, gains[0].Networth.BalanceAmount.Equal(decimal.NewFromInt(100)), "future postings must not inflate the gain balance, got %s", gains[0].Networth.BalanceAmount)
}
