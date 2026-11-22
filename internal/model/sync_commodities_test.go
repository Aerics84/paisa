package model

import (
	"errors"
	"testing"
	"time"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/model/price"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type stubProvider struct {
	code       string
	full       []*price.Price
	since      []*price.Price
	sinceArg   time.Time
	fullCalls  int
	sinceCalls int
	err        error
}

func (p *stubProvider) Code() string                                  { return p.code }
func (p *stubProvider) Label() string                                 { return "stub" }
func (p *stubProvider) Description() string                           { return "stub" }
func (p *stubProvider) AutoCompleteFields() []price.AutoCompleteField { return nil }
func (p *stubProvider) AutoComplete(*gorm.DB, string, map[string]string) []price.AutoCompleteItem {
	return nil
}
func (p *stubProvider) ClearCache(*gorm.DB) {}
func (p *stubProvider) GetPrices(string, string) ([]*price.Price, error) {
	p.fullCalls++
	return p.full, p.err
}
func (p *stubProvider) GetPricesSince(_ string, _ string, since time.Time) ([]*price.Price, error) {
	p.sinceCalls++
	p.sinceArg = since
	return p.since, p.err
}

func openSyncTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&price.Price{}))

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
	})

	return db
}

func syncTestDate(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func syncTestCommodity(name string, typ config.CommodityType, provider, code string) config.Commodity {
	return config.Commodity{
		Name:  name,
		Type:  typ,
		Price: config.Price{Provider: provider, Code: code},
	}
}

func stubProviderLookup(t *testing.T, mapping map[string]price.PriceProvider) {
	t.Helper()

	original := getProvider
	getProvider = func(code string) price.PriceProvider {
		return mapping[code]
	}
	t.Cleanup(func() {
		getProvider = original
	})
}

func seedSyncPrices(t *testing.T, db *gorm.DB, typ config.CommodityType, name, code string, dates ...time.Time) {
	t.Helper()

	prices := make([]*price.Price, 0, len(dates))
	for i, date := range dates {
		prices = append(prices, &price.Price{
			Date:  date,
			Value: decimal.NewFromInt(int64(100 + i)),
		})
	}
	require.NoError(t, price.UpsertMerged(db, typ, name, code, prices, true))
}

func syncTestPrices(t *testing.T, db *gorm.DB, typ config.CommodityType, name string) []price.Price {
	t.Helper()

	var rows []price.Price
	require.NoError(t, db.Where("commodity_type = ? and commodity_name = ?", typ, name).Order("date").Find(&rows).Error)
	return rows
}

func TestSyncCommoditiesUsesIncrementalSince(t *testing.T) {
	db := openSyncTestDB(t)
	now := syncTestDate(2026, 11, 24)
	seedSyncPrices(t, db, config.Stock, "GOLD", "gold",
		syncTestDate(2026, 11, 18), syncTestDate(2026, 11, 20))

	stub := &stubProvider{
		code:  "stub",
		since: []*price.Price{{Date: syncTestDate(2026, 11, 23), Value: decimal.NewFromInt(200)}},
	}
	stubProviderLookup(t, map[string]price.PriceProvider{"stub": stub})

	result := syncCommodities(db,
		[]config.Commodity{syncTestCommodity("GOLD", config.Stock, "stub", "gold")},
		SyncCommoditiesOptions{Full: false, Now: now, Concurrency: 1})

	require.Empty(t, result.Errors)
	assert.Equal(t, []string{"GOLD"}, result.Succeeded)
	assert.Equal(t, 1, stub.sinceCalls)
	assert.Equal(t, 0, stub.fullCalls)
	assert.Truef(t, stub.sinceArg.Equal(syncTestDate(2026, 11, 13)),
		"since = %s, want 2026-11-13", stub.sinceArg)

	rows := syncTestPrices(t, db, config.Stock, "GOLD")
	require.Len(t, rows, 3)
	assert.Equal(t, syncTestDate(2026, 11, 23), rows[2].Date.UTC())
}

func TestSyncCommoditiesFallsBackToFullWhenIncrementalEmpty(t *testing.T) {
	db := openSyncTestDB(t)
	now := syncTestDate(2026, 11, 24)
	seedSyncPrices(t, db, config.Stock, "GOLD", "gold", syncTestDate(2026, 11, 20))

	stub := &stubProvider{
		code: "stub",
		full: []*price.Price{
			{Date: syncTestDate(2026, 11, 18), Value: decimal.NewFromInt(200)},
			{Date: syncTestDate(2026, 11, 20), Value: decimal.NewFromInt(201)},
			{Date: syncTestDate(2026, 11, 23), Value: decimal.NewFromInt(202)},
		},
	}
	stubProviderLookup(t, map[string]price.PriceProvider{"stub": stub})

	result := syncCommodities(db,
		[]config.Commodity{syncTestCommodity("GOLD", config.Stock, "stub", "gold")},
		SyncCommoditiesOptions{Full: false, Now: now, Concurrency: 1})

	require.Empty(t, result.Errors)
	assert.Equal(t, 1, stub.sinceCalls)
	assert.Equal(t, 1, stub.fullCalls)

	rows := syncTestPrices(t, db, config.Stock, "GOLD")
	require.Len(t, rows, 3)
	assert.Equal(t, syncTestDate(2026, 11, 23), rows[2].Date.UTC())
	assert.True(t, rows[2].Value.Equal(decimal.NewFromInt(202)))
}

func TestSyncCommoditiesIncrementalFallbackWithoutRecentDataFails(t *testing.T) {
	db := openSyncTestDB(t)
	now := syncTestDate(2026, 11, 24)
	seedSyncPrices(t, db, config.Stock, "GOLD", "gold",
		syncTestDate(2026, 11, 18), syncTestDate(2026, 11, 20))

	stub := &stubProvider{
		code: "stub",
		full: []*price.Price{{Date: syncTestDate(2026, 11, 1), Value: decimal.NewFromInt(200)}},
	}
	stubProviderLookup(t, map[string]price.PriceProvider{"stub": stub})

	result := syncCommodities(db,
		[]config.Commodity{syncTestCommodity("GOLD", config.Stock, "stub", "gold")},
		SyncCommoditiesOptions{Full: false, Now: now, Concurrency: 1})

	require.Len(t, result.Failed, 1)
	assert.Equal(t, "GOLD", result.Failed[0])
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0], "GOLD")

	rows := syncTestPrices(t, db, config.Stock, "GOLD")
	require.Len(t, rows, 2)
	assert.Equal(t, syncTestDate(2026, 11, 20), rows[1].Date.UTC())
}

func TestSyncCommoditiesReportsUnknownProvider(t *testing.T) {
	db := openSyncTestDB(t)
	stubProviderLookup(t, nil)

	result := syncCommodities(db,
		[]config.Commodity{syncTestCommodity("X", config.Stock, "bogus", "x")},
		SyncCommoditiesOptions{Full: false, Now: syncTestDate(2026, 11, 24), Concurrency: 1})

	require.Len(t, result.Failed, 1)
	assert.Equal(t, "X", result.Failed[0])
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0], "X")
	assert.Contains(t, result.Errors[0], "bogus")
	assert.Empty(t, result.Succeeded)
	require.Error(t, result.Err())
}

func TestSyncCommoditiesContinuesAfterOneFailure(t *testing.T) {
	db := openSyncTestDB(t)
	now := syncTestDate(2026, 11, 24)

	good := &stubProvider{
		code: "good",
		full: []*price.Price{{Date: syncTestDate(2026, 11, 23), Value: decimal.NewFromInt(200)}},
	}
	bad := &stubProvider{code: "bad", err: errors.New("network down")}
	stubProviderLookup(t, map[string]price.PriceProvider{"good": good, "bad": bad})

	result := syncCommodities(db,
		[]config.Commodity{
			syncTestCommodity("GOOD", config.Stock, "good", "good"),
			syncTestCommodity("BAD", config.Stock, "bad", "bad"),
		},
		SyncCommoditiesOptions{Full: true, Now: now, Concurrency: 2})

	assert.ElementsMatch(t, []string{"GOOD"}, result.Succeeded)
	assert.ElementsMatch(t, []string{"BAD"}, result.Failed)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0], "BAD")
	assert.Contains(t, result.Errors[0], "network down")

	err := result.Err()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "network down")

	assert.Len(t, syncTestPrices(t, db, config.Stock, "GOOD"), 1)
	assert.Empty(t, syncTestPrices(t, db, config.Stock, "BAD"))
}

func TestSyncCommoditiesDoesNotDeleteOnEmptyFullResponse(t *testing.T) {
	db := openSyncTestDB(t)
	now := syncTestDate(2026, 11, 24)
	seedSyncPrices(t, db, config.Stock, "GOLD", "gold",
		syncTestDate(2026, 11, 18), syncTestDate(2026, 11, 20))

	stub := &stubProvider{code: "stub"}
	stubProviderLookup(t, map[string]price.PriceProvider{"stub": stub})

	result := syncCommodities(db,
		[]config.Commodity{syncTestCommodity("GOLD", config.Stock, "stub", "gold")},
		SyncCommoditiesOptions{Full: true, Now: now, Concurrency: 1})

	require.Len(t, result.Failed, 1)
	assert.Equal(t, "GOLD", result.Failed[0])
	require.Error(t, result.Err())

	rows := syncTestPrices(t, db, config.Stock, "GOLD")
	require.Len(t, rows, 2)
	assert.Equal(t, syncTestDate(2026, 11, 20), rows[1].Date.UTC())
}

func TestSyncCommoditiesFullRefreshReplacesHistory(t *testing.T) {
	db := openSyncTestDB(t)
	now := syncTestDate(2026, 11, 24)
	seedSyncPrices(t, db, config.Stock, "GOLD", "gold",
		syncTestDate(2026, 11, 18), syncTestDate(2026, 11, 20))

	stub := &stubProvider{
		code: "stub",
		full: []*price.Price{{Date: syncTestDate(2026, 11, 23), Value: decimal.NewFromInt(200)}},
	}
	stubProviderLookup(t, map[string]price.PriceProvider{"stub": stub})

	result := syncCommodities(db,
		[]config.Commodity{syncTestCommodity("GOLD", config.Stock, "stub", "gold")},
		SyncCommoditiesOptions{Full: true, Now: now, Concurrency: 1})

	require.Empty(t, result.Errors)
	rows := syncTestPrices(t, db, config.Stock, "GOLD")
	require.Len(t, rows, 1)
	assert.Equal(t, syncTestDate(2026, 11, 23), rows[0].Date.UTC())
}
