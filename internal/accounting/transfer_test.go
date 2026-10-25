package accounting

import (
	"testing"

	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsInternalTransfer(t *testing.T) {
	tx := []posting.Posting{
		{TransactionID: "t1", Account: "Assets:Cash"},
		{TransactionID: "t1", Account: "Assets:Broker"},
	}
	assert.True(t, IsInternalTransfer(tx, []string{"Assets:Cash"}))

	withExpense := append(append([]posting.Posting{}, tx...), posting.Posting{TransactionID: "t1", Account: "Expenses:Fee"})
	assert.False(t, IsInternalTransfer(withExpense, []string{"Assets:Cash"}))
	assert.False(t, IsInternalTransfer(tx, nil))

	assert.True(t, IsInternalTransfer([]posting.Posting{
		{TransactionID: "t2", Account: "Assets:Cash:Wallet"},
		{TransactionID: "t2", Account: "Assets:Broker"},
	}, []string{"Assets:Cash"}), "a plain account name should match its subaccounts")
}

func TestFilterOutInternalTransfersRemovesWholeTransaction(t *testing.T) {
	ps := []posting.Posting{
		{TransactionID: "transfer", Account: "Assets:Cash"},
		{TransactionID: "transfer", Account: "Assets:Broker"},
		{TransactionID: "buy", Account: "Assets:Checking"},
		{TransactionID: "buy", Account: "Assets:Broker"},
	}
	filtered := FilterOutInternalTransfers(ps, []string{"Assets:Cash"})
	require.Len(t, filtered, 2)
	assert.Equal(t, "buy", filtered[0].TransactionID)
	assert.Equal(t, "buy", filtered[1].TransactionID)
}
