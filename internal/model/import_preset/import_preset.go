package import_preset

import "fmt"

type PresetType string

const (
	Builtin PresetType = "builtin"
	Custom  PresetType = "custom"
)

// ImportPreset is the API shape returned to frontend clients.
type ImportPreset struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	ColumnMappings  map[string]string `json:"column_mappings"`
	DateFormat      string            `json:"date_format"`
	DefaultAccounts map[string]string `json:"default_accounts"`
	Delimiter       string            `json:"delimiter"`
	PresetType      PresetType        `json:"preset_type"`
}

func BuiltinPresets() []ImportPreset {
	return []ImportPreset{
		{
			ID:   buildID("Kotak Bank Statement CSV", Builtin),
			Name: "Kotak Bank Statement CSV",
			ColumnMappings: map[string]string{
				"date": "B", "description": "D", "amount": "F", "dr_cr": "G", "balance": "H",
			},
			DateFormat:      "DD-MM-YY",
			DefaultAccounts: map[string]string{"asset": "Assets:Checking:Kotak"},
			Delimiter:       ",",
			PresetType:      Builtin,
		},
		{
			ID:   buildID("ICICI Bank Statement CSV", Builtin),
			Name: "ICICI Bank Statement CSV",
			ColumnMappings: map[string]string{
				"date": "A", "description": "B", "debit": "C", "credit": "D", "balance": "E",
			},
			DateFormat:      "DD/MM/YYYY",
			DefaultAccounts: map[string]string{"asset": "Assets:Checking:ICICI"},
			Delimiter:       ",",
			PresetType:      Builtin,
		},
	}
}

func buildID(name string, presetType PresetType) string {
	return fmt.Sprintf("%s:%s", presetType, name)
}
