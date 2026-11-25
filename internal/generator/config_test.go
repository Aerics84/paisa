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

func TestMinimalConfigDefaultsToGermany(t *testing.T) {
	dir := t.TempDir()

	require.NoError(t, MinimalConfig(dir))

	configContent, err := os.ReadFile(filepath.Join(dir, "paisa.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(configContent), "regional_profile: germany-eu")
}

func TestMinimalConfigForProfileIndiaWritesIndia(t *testing.T) {
	dir := t.TempDir()

	require.NoError(t, MinimalConfigForProfile(dir, config.RegionalProfileIndia))

	configContent, err := os.ReadFile(filepath.Join(dir, "paisa.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(configContent), "regional_profile: india")
}

func TestMinimalConfigForGermanyEUWritesRegionalProfile(t *testing.T) {
	dir := t.TempDir()

	require.NoError(t, MinimalConfigForProfile(dir, config.RegionalProfileGermanyEU))

	configContent, err := os.ReadFile(filepath.Join(dir, "paisa.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(configContent), "regional_profile: germany-eu")
}

func TestMinimalConfigForProfilePreservesExistingJournal(t *testing.T) {
	dir := t.TempDir()
	journalPath := filepath.Join(dir, "main.ledger")
	existing := []byte("2024/01/01 Opening balance\n    Assets:Checking:ING    100,00 EUR\n    Equity:Opening\n")
	require.NoError(t, os.WriteFile(journalPath, existing, 0o644))

	require.NoError(t, MinimalConfigForProfile(dir, config.RegionalProfileGermanyEU))

	journalContent, err := os.ReadFile(journalPath)
	require.NoError(t, err)
	assert.Equal(t, existing, journalContent, "auto-created config must not truncate an existing journal")
}

func TestMinimalConfigForProfileCreatesMissingJournal(t *testing.T) {
	dir := t.TempDir()
	journalPath := filepath.Join(dir, "main.ledger")

	require.NoError(t, MinimalConfigForProfile(dir, config.RegionalProfileGermanyEU))

	info, err := os.Stat(journalPath)
	require.NoError(t, err)
	assert.Zero(t, info.Size())
}

func TestGermanyEUDemoUsesEuropeanStarterContent(t *testing.T) {
	dir := t.TempDir()
	legacySheet := filepath.Join(dir, "Schedule AL.paisa")
	require.NoError(t, os.WriteFile(legacySheet, []byte("legacy"), 0644))

	require.NoError(t, DemoForProfile(dir, config.RegionalProfileGermanyEU))

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
