package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGermanyEURegionalProfileAppliesDefaults(t *testing.T) {
	err := LoadConfig([]byte(`
journal_path: main.ledger
db_path: paisa.db
regional_profile: germany-eu
`), "")
	require.NoError(t, err)

	cfg := GetConfig()
	assert.Equal(t, RegionalProfileGermanyEU, cfg.RegionalProfile)
	assert.Equal(t, TaxRegimeGermany, cfg.TaxRegime)
	assert.Equal(t, "EUR", cfg.DefaultCurrency)
	assert.Equal(t, 2, cfg.DisplayPrecision)
	assert.Equal(t, "de-DE", cfg.Locale)
	assert.Equal(t, "Europe/Berlin", cfg.TimeZone)
	assert.EqualValues(t, 1, cfg.FinancialYearStartingMonth)
	assert.EqualValues(t, 1, cfg.WeekStartingDay)
	assert.Equal(t, 1000.0, cfg.GermanyTax.AnnualAllowance)
	assert.Equal(t, 0.25, cfg.GermanyTax.CapitalIncomeTaxRate)
	assert.Equal(t, 0.055, cfg.GermanyTax.SolidaritySurchargeRate)
	assert.Equal(t, 0.0, cfg.GermanyTax.ChurchTaxRate)
}

func TestRegionalProfileRespectsExplicitOverrides(t *testing.T) {
	err := LoadConfig([]byte(`
journal_path: main.ledger
db_path: paisa.db
regional_profile: germany-eu
tax_regime: india
locale: en-GB
time_zone: ""
display_precision: 3
financial_year_starting_month: 4
week_starting_day: 0
`), "")
	require.NoError(t, err)

	cfg := GetConfig()
	assert.Equal(t, RegionalProfileGermanyEU, cfg.RegionalProfile)
	assert.Equal(t, TaxRegimeIndia, cfg.TaxRegime)
	assert.Equal(t, "en-GB", cfg.Locale)
	assert.Equal(t, "", cfg.TimeZone)
	assert.Equal(t, 3, cfg.DisplayPrecision)
	assert.EqualValues(t, 4, cfg.FinancialYearStartingMonth)
	assert.EqualValues(t, 0, cfg.WeekStartingDay)
}

func TestIndiaTaxRegimeSupportsCurrentTaxFeatures(t *testing.T) {
	err := LoadConfig([]byte(`
journal_path: main.ledger
db_path: paisa.db
tax_regime: india
`), "")
	require.NoError(t, err)

	assert.True(t, SupportsTaxFeatures())
	assert.True(t, SupportsIndiaTaxFeatures())
	assert.True(t, SupportsScheduleAL())
	assert.False(t, SupportsGermanyTaxFeatures())
}

func TestGermanyTaxRegimeDisablesIndiaOnlyTaxFeatures(t *testing.T) {
	err := LoadConfig([]byte(`
journal_path: main.ledger
db_path: paisa.db
tax_regime: germany
`), "")
	require.NoError(t, err)

	assert.True(t, SupportsTaxFeatures())
	assert.False(t, SupportsIndiaTaxFeatures())
	assert.False(t, SupportsScheduleAL())
	assert.True(t, SupportsGermanyTaxFeatures())
}
