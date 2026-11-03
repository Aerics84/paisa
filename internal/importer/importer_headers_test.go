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

	candidates, err := parseCSVRows(content, ",", "", nil, "de-DE")
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

func TestParseCSVGermanBuchungHeader(t *testing.T) {
	content := `Buchung;Auftraggeber;Betrag;Saldo
02.11.2026;REWE;-50,00;1.000,00
`

	candidates, err := parseCSVRows(content, ",", "", nil, "de-DE")
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	assert.Equal(t, "2026-11-02", candidates[0].Date)
	assert.Equal(t, "REWE", candidates[0].Payee)
	require.NotNil(t, candidates[0].Balance)
	assert.InDelta(t, 1000, *candidates[0].Balance, 0.001)
}
