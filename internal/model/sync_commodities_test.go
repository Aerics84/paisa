package model

import (
	"testing"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/model/price"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSyncCommoditiesReportsUnknownProviderWithoutPanic(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:sync-commodities?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&price.Price{}))

	commodities := []config.Commodity{{
		Name:  "X",
		Type:  config.Stock,
		Price: config.Price{Provider: "bogus", Code: "x"},
	}}

	require.NotPanics(t, func() {
		err = syncCommodities(db, commodities)
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "X")
	assert.Contains(t, err.Error(), "bogus")
}
