package ledger

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBeancountReadsPostingMetaOnly(t *testing.T) {
	if _, err := exec.LookPath("bean-query"); err != nil {
		t.Skip("bean-query executable is not installed")
	}

	dir := t.TempDir()
	journal := filepath.Join(dir, "main.beancount")
	require.NoError(t, os.WriteFile(journal, []byte(`
2025-09-09 * "PayPal" "Bürostuhl"
  Assets:Checking:ING       -699.00 EUR
  Expenses:Household:Goods   699.00 EUR
    tax_category: "work_equipment"

2025-10-01 * "Split" "Two expenses"
  tax_category: "health"
  Assets:Checking:ING       -100.00 EUR
  Expenses:Health             60.00 EUR
  Expenses:Health:Medicine    40.00 EUR
`), 0600))

	require.NoError(t, config.LoadConfig([]byte(`
journal_path: `+journal+`
db_path: `+filepath.Join(dir, "paisa.db")+`
ledger_cli: beancount
tax_regime: germany
default_currency: EUR
time_zone: Europe/Berlin
`), journal))

	postings, err := (Beancount{}).Parse(journal, nil)
	require.NoError(t, err)

	seen := map[string]bool{}
	for _, p := range postings {
		seen[p.Account] = true
		if p.Account == "Expenses:Household:Goods" {
			assert.Equal(t, "work_equipment", p.TaxCategory)
		}
		if p.Account == "Expenses:Health" || p.Account == "Expenses:Health:Medicine" {
			assert.Empty(t, p.TaxCategory, "transaction metadata must not leak onto postings")
		}
	}
	require.True(t, seen["Expenses:Household:Goods"], "expected tagged posting was not parsed")
	require.True(t, seen["Expenses:Health"], "expected split posting was not parsed")
	require.True(t, seen["Expenses:Health:Medicine"], "expected split posting was not parsed")
}
