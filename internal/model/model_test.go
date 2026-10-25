package model

import (
	"testing"

	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/ananthakumaran/paisa/internal/model/price"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestPostingAndPriceIndexesExist(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:indexes?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&posting.Posting{}, &price.Price{}))

	names := func(table string) []string {
		var indexes []struct{ Name string }
		require.NoError(t, db.Raw("PRAGMA index_list('"+table+"')").Scan(&indexes).Error)
		out := make([]string, 0, len(indexes))
		for _, index := range indexes {
			out = append(out, index.Name)
		}
		return out
	}

	assert.Contains(t, names("postings"), "idx_postings_date")
	assert.Contains(t, names("postings"), "idx_postings_account")
	assert.Contains(t, names("postings"), "idx_postings_commodity")
	assert.Contains(t, names("prices"), "idx_prices_date")
	assert.Contains(t, names("prices"), "idx_prices_commodity_name")
}
