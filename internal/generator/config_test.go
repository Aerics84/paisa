package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMinimalConfigForGermanyEUWritesRegionalProfile(t *testing.T) {
	dir := t.TempDir()

	MinimalConfigForProfile(dir, config.RegionalProfileGermanyEU)

	configContent, err := os.ReadFile(filepath.Join(dir, "paisa.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(configContent), "regional_profile: germany-eu")
}

func TestGermanyEUDemoUsesEuropeanStarterContent(t *testing.T) {
	dir := t.TempDir()
	legacySheet := filepath.Join(dir, "Schedule AL.paisa")
	require.NoError(t, os.WriteFile(legacySheet, []byte("legacy"), 0644))

	DemoForProfile(dir, config.RegionalProfileGermanyEU)

	configContent, err := os.ReadFile(filepath.Join(dir, "paisa.yaml"))
	require.NoError(t, err)
	configString := string(configContent)
	assert.Contains(t, configString, "regional_profile: germany-eu")
	assert.Contains(t, configString, "default_currency: EUR")
	assert.Contains(t, configString, "tax_regime: germany")
	assert.NotContains(t, configString, "schedule_al:")
	assert.NotContains(t, configString, "NIFTY")
	assert.NotContains(t, configString, "SBI")

	journalContent, err := os.ReadFile(filepath.Join(dir, "main.ledger"))
	require.NoError(t, err)
	journalString := string(journalContent)
	assert.Contains(t, journalString, "Acme GmbH")
	assert.Contains(t, journalString, "Assets:Checking:ING")
	assert.Contains(t, journalString, "Assets:Equity:VWCE")
	assert.Contains(t, journalString, "EUR")
	assert.NotContains(t, journalString, "INR")
	assert.NotContains(t, journalString, "NPS_HDFC")

	_, err = os.Stat(legacySheet)
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestNormalizeRegionalProfileFallsBackToIndia(t *testing.T) {
	assert.Equal(t, config.RegionalProfileIndia, config.NormalizeRegionalProfile(""))
	assert.Equal(t, config.RegionalProfileIndia, config.NormalizeRegionalProfile("unsupported"))
	assert.True(t, config.IsSupportedRegionalProfile(config.RegionalProfileGermanyEU))
	assert.False(t, config.IsSupportedRegionalProfile(config.RegionalProfileType(strings.ToUpper("germany-eu"))))
}
