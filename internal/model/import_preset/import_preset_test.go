package import_preset

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuiltinPresetsContainsICICIBankStatement(t *testing.T) {
	var icici *ImportPreset
	for i, preset := range BuiltinPresets() {
		if preset.Name == "ICICI Bank Statement CSV" {
			icici = &BuiltinPresets()[i]
			break
		}
	}
	require.NotNil(t, icici)

	assert.Equal(t, "builtin:ICICI Bank Statement CSV", icici.ID)
	assert.Equal(t, map[string]string{
		"date":        "A",
		"description": "B",
		"debit":       "C",
		"credit":      "D",
		"balance":     "E",
	}, icici.ColumnMappings)
	assert.Equal(t, "DD/MM/YYYY", icici.DateFormat)
	assert.Equal(t, map[string]string{"asset": "Assets:Checking:ICICI"}, icici.DefaultAccounts)
	assert.Equal(t, ",", icici.Delimiter)
	assert.Equal(t, Builtin, icici.PresetType)
}

func TestBuiltinPresetsContainsKotakStatement(t *testing.T) {
	var kotak *ImportPreset
	for i, preset := range BuiltinPresets() {
		if preset.Name == "Kotak Bank Statement CSV" {
			kotak = &BuiltinPresets()[i]
			break
		}
	}
	require.NotNil(t, kotak)

	assert.Equal(t, "builtin:Kotak Bank Statement CSV", kotak.ID)
	assert.Equal(t, map[string]string{
		"date":        "B",
		"description": "D",
		"amount":      "F",
		"dr_cr":       "G",
		"balance":     "H",
	}, kotak.ColumnMappings)
	assert.Equal(t, "DD-MM-YY", kotak.DateFormat)
	assert.Equal(t, map[string]string{"asset": "Assets:Checking:Kotak"}, kotak.DefaultAccounts)
	assert.Equal(t, ",", kotak.Delimiter)
	assert.Equal(t, Builtin, kotak.PresetType)
}
