package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
