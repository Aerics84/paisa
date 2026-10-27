package ledger

import (
	"testing"
	"time"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHLedgerReadsTaxCategoryAfterRecurringTag(t *testing.T) {
	require.NoError(t, config.LoadConfig([]byte("journal_path: main.ledger\ndb_path: test.db\ndefault_currency: EUR\ntime_zone: UTC\n"), ""))
	date := time.Date(2025, 9, 9, 0, 0, 0, 0, time.UTC)
	p := HLedgerPosting{
		Account: "Expenses:Household:Goods",
		Tags: [][]string{
			{"Recurring", "Office"},
			{"tax_category", "work_equipment"},
			{"TaxRelevant", "Work"},
		},
	}
	p.Amount = append(p.Amount, struct {
		Commodity string `json:"acommodity"`
		Quantity  struct {
			Value float64 `json:"floatingPoint"`
		} `json:"aquantity"`
		Price hledgerPrice `json:"aprice"`
		Cost  hledgerPrice `json:"acost"`
	}{Commodity: "EUR"})
	p.Amount[0].Quantity.Value = 699
	tx := HLedgerTransaction{Description: "PayPal | Bürostuhl", Status: "Cleared"}
	tx.TSourcePos = append(tx.TSourcePos,
		struct {
			SourceColumn uint64 `json:"sourceColumn"`
			SourceLine   uint64 `json:"sourceLine"`
			SourceName   string `json:"sourceName"`
		}{SourceName: "main.ledger", SourceLine: 1},
		struct {
			SourceColumn uint64 `json:"sourceColumn"`
			SourceLine   uint64 `json:"sourceLine"`
			SourceName   string `json:"sourceName"`
		}{SourceName: "main.ledger", SourceLine: 3},
	)

	postings, err := buildHLedgerPostings(p, tx, nil, date)
	require.NoError(t, err)
	require.Len(t, postings, 1)
	assert.Equal(t, "work_equipment", postings[0].TaxCategory)
	assert.Equal(t, "Work", postings[0].TaxCategoryLegacy)
}
