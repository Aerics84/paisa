package ledger

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLedgerCLIParsesTaxCategoryAndLegacyTag(t *testing.T) {
	if _, err := exec.LookPath("ledger"); err != nil {
		t.Skip("ledger executable is not installed")
	}

	dir := t.TempDir()
	journal := filepath.Join(dir, "main.ledger")
	require.NoError(t, os.WriteFile(journal, []byte(`commodity EUR
	format 1.000,00 EUR

2025/09/09 * PayPal ; Bürostuhl
    Assets:Checking:ING                      -699,00 EUR
    Expenses:Household:Goods                  699,00 EUR ; tax_category: work_equipment

2025/06/02 * ADAC
    Assets:Checking:ING                       -41,00 EUR ; TaxRelevant: Auto
    Expenses:Car:Memberships                   41,00 EUR
`), 0600))

	require.NoError(t, config.LoadConfig([]byte(`
journal_path: `+journal+`
db_path: `+filepath.Join(dir, "paisa.db")+`
tax_regime: germany
default_currency: EUR
time_zone: Europe/Berlin
`), ""))

	postings, err := (LedgerCLI{}).Parse(journal, nil)
	require.NoError(t, err)

	var chair, chairBank, adacBank, adacExpense *posting.Posting
	for _, p := range postings {
		switch {
		case p.Account == "Expenses:Household:Goods":
			chair = p
		case p.Account == "Assets:Checking:ING" && strings.Contains(p.Payee, "PayPal"):
			chairBank = p
		case p.Account == "Assets:Checking:ING" && p.Payee == "ADAC":
			adacBank = p
		case p.Account == "Expenses:Car:Memberships":
			adacExpense = p
		}
	}
	require.NotNil(t, chair)
	require.NotNil(t, chairBank)
	require.NotNil(t, adacBank)
	require.NotNil(t, adacExpense)

	// Posting-level extraction: the tag stays on the posting that carries it.
	assert.Equal(t, "work_equipment", chair.TaxCategory)
	assert.Empty(t, chair.TaxCategoryLegacy)
	assert.Empty(t, chairBank.TaxCategory)
	assert.Equal(t, "Auto", adacBank.TaxCategoryLegacy)
	assert.Empty(t, adacExpense.TaxCategoryLegacy)
	assert.Empty(t, adacExpense.TaxCategory)
}
