package importer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectCSVDelimiter(t *testing.T) {
	assert.Equal(t, ',', detectCSVDelimiter("a,b,c\n1,2,3", ""))
	assert.Equal(t, ';', detectCSVDelimiter("a;b;c\n1;2;3", ","))
	assert.Equal(t, '\t', detectCSVDelimiter("a\tb\tc\n1\t2\t3", ","))
	assert.Equal(t, ';', detectCSVDelimiter("a;b;c\n1;2;3", ";"))
	assert.Equal(t, ',', detectCSVDelimiter("single-column", ","))
}

func TestParseCSVGermanSemicolonHeaders(t *testing.T) {
	content := `Umsatzanzeige
Konto;DE81500105175446403819
Buchungstag;Verwendungszweck;Betrag;Saldo;Währung
02.11.2026;REWE;-50,00;1.000,00;EUR
03.11.2026;Gehalt;2.000,00;3.000,00;EUR
`

	candidates, err := parseCSVRows(content, ",", "", nil, "de-DE", nil)
	require.NoError(t, err)
	require.Len(t, candidates, 2)

	assert.Equal(t, "2026-11-02", candidates[0].Date)
	assert.InDelta(t, 50, candidates[0].Amount, 0.001)
	assert.True(t, candidates[0].IsDebit)
	require.NotNil(t, candidates[0].Balance)
	assert.InDelta(t, 1000, *candidates[0].Balance, 0.001)

	assert.Equal(t, "2026-11-03", candidates[1].Date)
	assert.InDelta(t, 2000, candidates[1].Amount, 0.001)
	assert.False(t, candidates[1].IsDebit)
	require.NotNil(t, candidates[1].Balance)
	assert.InDelta(t, 3000, *candidates[1].Balance, 0.001)
}

func TestParseCSVExplicitColumnMapping(t *testing.T) {
	content := `Export
Kol1;Kol2;Kol3;Kol4;Kol5
02.11.2026;Händler X;-41,43;9.020,59;EUR
`
	mapping := &IngestionColumnMapping{
		HeaderRow:   1,
		Date:        0,
		Description: 1,
		Amount:      2,
		Balance:     3,
		Currency:    4,
		Debit:       -1,
		Credit:      -1,
		DrCr:        -1,
	}

	candidates, err := parseCSVRows(content, ";", "", nil, "de-DE", mapping)
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	assert.Equal(t, "2026-11-02", candidates[0].Date)
	assert.Equal(t, "Händler X", candidates[0].Payee)
	assert.InDelta(t, 41.43, candidates[0].Amount, 0.001)
	require.NotNil(t, candidates[0].Balance)
	assert.InDelta(t, 9020.59, *candidates[0].Balance, 0.001)
}

func TestParseCSVPrefersMerchantAsPayee(t *testing.T) {
	content := `Buchung;Auftraggeber;Buchungstext;Betrag;Saldo
02.11.2026;VISA METZGEREI RALF OTT;Lastschrift;-41,43;9.020,59
`

	candidates, err := parseCSVRows(content, ";", "", nil, "de-DE", nil)
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	assert.Equal(t, "VISA METZGEREI RALF OTT", candidates[0].Payee)
	assert.Equal(t, "Lastschrift", candidates[0].Memo)
}

func TestParseCSVGermanBuchungHeader(t *testing.T) {
	content := `Buchung;Auftraggeber;Betrag;Saldo
02.11.2026;REWE;-50,00;1.000,00
`

	candidates, err := parseCSVRows(content, ",", "", nil, "de-DE", nil)
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	assert.Equal(t, "2026-11-02", candidates[0].Date)
	assert.Equal(t, "REWE", candidates[0].Payee)
	require.NotNil(t, candidates[0].Balance)
	assert.InDelta(t, 1000, *candidates[0].Balance, 0.001)
}
