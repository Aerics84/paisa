package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ananthakumaran/paisa/internal/accounting"
	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/model/cache"
	"github.com/ananthakumaran/paisa/internal/model/portfolio"
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/ananthakumaran/paisa/internal/model/price"
	"github.com/ananthakumaran/paisa/internal/service"
	"github.com/ananthakumaran/paisa/internal/utils"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const assetsContractConfig = `journal_path: main.ledger
db_path: paisa.db
default_currency: EUR
time_zone: UTC
`

func openAssetsContractDB(t *testing.T) *gorm.DB {
	t.Helper()

	name := strings.NewReplacer("/", "_", "\\", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&posting.Posting{},
		&price.Price{},
		&portfolio.Portfolio{},
		&cache.Cache{},
	))

	service.ClearInterestCache()
	service.ClearPriceCache()
	accounting.ClearCache()
	t.Cleanup(service.ClearInterestCache)
	t.Cleanup(service.ClearPriceCache)
	t.Cleanup(accounting.ClearCache)

	return db
}

func loadAssetsContractConfig(t *testing.T, content string) {
	t.Helper()

	require.NoError(t, config.LoadConfig([]byte(content), ""))
	t.Cleanup(func() {
		_ = config.LoadConfig([]byte(assetsContractConfig), "")
	})
}

func assetsContractRouter(t *testing.T, db *gorm.DB, content string) *ginEngine {
	t.Helper()

	loadAssetsContractConfig(t, content)
	store := config.NewStore()
	require.NoError(t, store.Load([]byte(content), ""))

	return &ginEngine{router: Build(Deps{DB: db, Config: store}, false)}
}

// ginEngine keeps the test body readable without importing gin everywhere.
type ginEngine struct {
	router http.Handler
}

func (e *ginEngine) get(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

// TestNetworthJSONContract pins the wire format of GET /api/networth. The
// fixture is checked in so that renaming a JSON field or changing a date from a
// string to an object cannot silently break the generated TypeScript types.
func TestNetworthJSONContract(t *testing.T) {
	utils.SetNow("2026-11-22")
	t.Cleanup(utils.ResetNow)

	db := openAssetsContractDB(t)
	require.NoError(t, db.Create(&posting.Posting{
		ID:            1,
		TransactionID: "fixture-networth",
		Date:          time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC),
		Payee:         "Fixture Buy",
		Account:       "Assets:Broker:Fund",
		Commodity:     "EUR",
		Quantity:      decimal.NewFromInt(100),
		Amount:        decimal.NewFromInt(100),
		Status:        "cleared",
		FileName:      "fixture.ledger",
	}).Error)

	engine := assetsContractRouter(t, db, assetsContractConfig)
	rec := engine.get(t, "/api/networth")
	require.Equal(t, http.StatusOK, rec.Code)

	var body NetworthResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))

	expected, err := os.ReadFile("testdata/api/networth.json")
	require.NoError(t, err)
	require.JSONEq(t, string(expected), rec.Body.String())
}

// TestAllocationJSONContract pins the wire format of GET /api/allocation,
// including the nested target aggregates and the derived rebalance plan.
func TestAllocationJSONContract(t *testing.T) {
	utils.SetNow("2026-11-22")
	t.Cleanup(utils.ResetNow)

	db := openAssetsContractDB(t)
	require.NoError(t, db.Create(&posting.Posting{
		ID:            1,
		TransactionID: "fixture-allocation",
		Date:          time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC),
		Payee:         "Fixture Buy",
		Account:       "Assets:Equity:Stock",
		Commodity:     "EUR",
		Quantity:      decimal.NewFromInt(1000),
		Amount:        decimal.NewFromInt(1000),
		Status:        "cleared",
		FileName:      "fixture.ledger",
	}).Error)

	content := assetsContractConfig + `allocation_targets:
  - name: Equity
    target: 100
    accounts:
      - "Assets:Equity:*"
`
	engine := assetsContractRouter(t, db, content)
	rec := engine.get(t, "/api/allocation")
	require.Equal(t, http.StatusOK, rec.Code)

	var body AllocationResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))

	expected, err := os.ReadFile("testdata/api/allocation.json")
	require.NoError(t, err)
	require.JSONEq(t, string(expected), rec.Body.String())
}

// TestPortfolioAssetResponsesHaveNoNullOnEmptyDB pins the null-vs-empty
// contract of the assets endpoints. The generated TypeScript types declare
// arrays and maps, so an empty journal must not produce JSON null.
func TestPortfolioAssetResponsesHaveNoNullOnEmptyDB(t *testing.T) {
	utils.SetNow("2026-11-22")
	t.Cleanup(utils.ResetNow)

	db := openAssetsContractDB(t)
	engine := assetsContractRouter(t, db, assetsContractConfig)

	for _, path := range []string{
		"/api/networth",
		"/api/investment",
		"/api/gain",
		"/api/gain/Assets:Broker",
		"/api/allocation",
		"/api/portfolio_allocation",
		"/api/assets/balance",
	} {
		rec := engine.get(t, path)
		require.Equal(t, http.StatusOK, rec.Code, path)

		var body map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), path)
		assertNoNull(t, body, path)
	}
}
