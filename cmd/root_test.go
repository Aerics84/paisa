package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFirstRunConfigPreservesExistingJournal(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "paisa.yaml")
	journalPath := filepath.Join(dir, "main.ledger")
	existing := []byte("2024/01/01 Opening balance\n    Assets:Checking:ING    100,00 EUR\n    Equity:Opening\n")
	require.NoError(t, os.WriteFile(journalPath, existing, 0o644))

	require.NoError(t, createFirstRunConfig(dir, configPath))

	journalContent, err := os.ReadFile(journalPath)
	require.NoError(t, err)
	assert.Equal(t, existing, journalContent, "first-run auto-create must not truncate an existing journal")

	configContent, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(configContent), "journal_path: 'main.ledger'")
	assert.Equal(t, "main.ledger", config.GetConfig().JournalPath)

	t.Cleanup(func() {
		config.DefaultStore().MarkInitialized()
	})
}

func TestFirstRunConfigDefaultsToGermanyEUAndNeedsOnboarding(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "paisa.yaml")

	require.NoError(t, createFirstRunConfig(dir, configPath))

	content, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "regional_profile: germany-eu")

	cfg := config.GetConfig()
	assert.Equal(t, config.RegionalProfileGermanyEU, cfg.RegionalProfile)
	assert.Equal(t, "EUR", cfg.DefaultCurrency)
	assert.Equal(t, config.TaxRegimeGermany, cfg.TaxRegime)

	assert.True(t, config.DefaultStore().AutoCreated())
	assert.False(t, config.DefaultStore().Initialized())

	t.Cleanup(func() {
		config.DefaultStore().MarkInitialized()
	})
}
