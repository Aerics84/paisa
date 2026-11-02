package taxation

import (
	"strings"
	"testing"
	"time"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/model/cache"
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/ananthakumaran/paisa/internal/model/price"
	"github.com/ananthakumaran/paisa/internal/service"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// taxTestDate returns a date-only value in the same location the taxation
// package constants are parsed in, so boundary comparisons stay deterministic.
func taxTestDate(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, config.TimeZone())
}

// taxTestDB opens an in-memory database with a single fair market value price
// on 2018-01-31, the day before the equity grandfathering cut-off.
func taxTestDB(t *testing.T, commodityName, fmv string) *gorm.DB {
	t.Helper()

	require.NoError(t, config.LoadConfig([]byte(`
journal_path: main.ledger
db_path: paisa.db
default_currency: INR
`), ""))

	name := strings.NewReplacer("/", "_", "\\", "_", " ", "_", ":", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:tax-"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&posting.Posting{}, &price.Price{}, &cache.Cache{}))

	require.NoError(t, db.Create(&price.Price{
		Date:          taxTestDate(2018, time.January, 31),
		CommodityName: commodityName,
		Value:         decimal.RequireFromString(fmv),
	}).Error)

	service.ClearPriceCache()
	t.Cleanup(service.ClearPriceCache)

	return db
}
