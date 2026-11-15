package ledger

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFilterVirtualLedgerRecordsSkipsVirtualPostings(t *testing.T) {
	records := [][]string{
		{"2026/01/01", "Assets:Bank", ""},
		{"2026/01/02", "[Assets:Goals:Car]", "1"},
		{"2026/01/03", "(Assets:Envelope)", "1"},
		{"2026/01/04", "Expenses:Food", ""},
		{"2026/01/05", "Assets:Short"},
	}

	filtered := filterVirtualLedgerRecords(records, 2)

	assert.Equal(t, [][]string{
		{"2026/01/01", "Assets:Bank", ""},
		{"2026/01/04", "Expenses:Food", ""},
		{"2026/01/05", "Assets:Short"},
	}, filtered)
}
