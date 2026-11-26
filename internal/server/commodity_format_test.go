package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/stretchr/testify/require"
)

func TestMissingCommodityFormats(t *testing.T) {
	dir := t.TempDir()
	commodities := filepath.Join(dir, "commodities.ledger")
	main := filepath.Join(dir, "main.ledger")

	require.NoError(t, os.WriteFile(commodities, []byte(`commodity "FORMATTED"
    format 1.000,00000 "FORMATTED"

commodity UNQUOTED
    format 1.000,00000 UNQUOTED

commodity 1.000,00000 "SINGLELINE"
`), 0600))

	require.NoError(t, os.WriteFile(main, []byte(`include commodities.ledger

2026/01/01 * Test
    Assets:Formatted   1,234567 "FORMATTED"
    Assets:Unquoted    1,234567 "UNQUOTED"
    Assets:Missing     1,234567 "MISSING"
    Assets:Single      1,234567 "SINGLELINE"
    Assets:Reversed    "REVERSED" 1,234567
    Assets:Five        1,23456 "MISSING"
    Assets:Priced      1,234567 "MISSING" @ 9,45 EUR
    ; comment 1,234567 "COMMENT"

2026/01/02 * Second
    Assets:Missing     2,345678 "MISSING"
`), 0600))

	errs := missingCommodityFormats([]string{main, commodities})
	require.Len(t, errs, 3)

	messages := make([]string, 0, len(errs))
	for _, err := range errs {
		messages = append(messages, err.Error())
	}
	joined := strings.Join(messages, "\n")

	require.Contains(t, joined, "MISSING")
	require.Contains(t, joined, "SINGLELINE")
	require.Contains(t, joined, "REVERSED")
	require.NotContains(t, joined, "FORMATTED")
	require.NotContains(t, joined, "UNQUOTED")
	require.NotContains(t, joined, "COMMENT")
	require.Contains(t, joined, "commodity")
	require.Contains(t, joined, "format")
}

func TestCommodityFormatRuleSkipsOtherParsers(t *testing.T) {
	store := config.NewStore()
	require.NoError(t, store.Load([]byte("journal_path: main.ledger\ndb_path: paisa.db\nledger_cli: hledger\n"), ""))

	require.Empty(t, ruleCommodityFormatMissing(Deps{Config: store}))
}
