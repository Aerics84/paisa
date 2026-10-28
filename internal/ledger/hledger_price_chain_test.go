package ledger

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const chainJournal = `P 2024-01-01 USD 1.10 EUR
P 2024-01-01 EUR 90 INR

2024-01-02 Chain
    Assets:A    1 USD @ 1.10 EUR
    Assets:B    -1.10 EUR
`

func setupChainJournal(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	journal := filepath.Join(dir, "main.ledger")
	require.NoError(t, os.WriteFile(journal, []byte(chainJournal), 0o644))
	require.NoError(t, config.LoadConfig(
		[]byte("journal_path: "+journal+"\ndb_path: test.db\ndefault_currency: INR\ntime_zone: UTC\n"),
		"",
	))
	return journal
}

func TestHLedgerPricesParseChain(t *testing.T) {
	journal := setupChainJournal(t)

	prices, err := (HLedgerCLI{}).Prices(journal)
	require.NoError(t, err)

	// parseHLedgerPrices keeps only directives quoted in the default currency,
	// so USD->EUR is filtered and EUR->INR remains.
	require.Len(t, prices, 1)
	assert.Equal(t, "EUR", prices[0].CommodityName)
	assert.True(t, prices[0].Value.Equal(decimal.RequireFromString("90")), "EUR price is 90 INR")
}

func TestHLedgerPostingUsesChainedPrice(t *testing.T) {
	journal := setupChainJournal(t)

	prices, err := (HLedgerCLI{}).Prices(journal)
	require.NoError(t, err)

	postings, err := (HLedgerCLI{}).Parse(journal, prices)
	require.NoError(t, err)

	var amount decimal.Decimal
	found := false
	for _, posting := range postings {
		if posting.Account == "Assets:A" {
			amount = posting.Amount
			found = true
			break
		}
	}
	require.True(t, found, "Assets:A posting exists")
	// 1 USD * 1.10 EUR/USD * 90 INR/EUR
	assert.True(t, amount.Equal(decimal.RequireFromString("99")), "chained amount is 99.00 INR, got %s", amount)
}
