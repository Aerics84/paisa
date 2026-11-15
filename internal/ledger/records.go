package ledger

// filterVirtualLedgerRecords drops postings that ledger marks as virtual
// ([Account], (Account)). They are not real money and must not show up in
// account lists, balances or editor autocomplete.
func filterVirtualLedgerRecords(records [][]string, virtualColumn int) [][]string {
	filtered := make([][]string, 0, len(records))
	for _, record := range records {
		if virtualColumn < len(record) && record[virtualColumn] == "1" {
			continue
		}
		filtered = append(filtered, record)
	}
	return filtered
}
