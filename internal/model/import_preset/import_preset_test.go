package import_preset

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
