package server

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ananthakumaran/paisa/internal/accounting"
	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/model/cache"
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/ananthakumaran/paisa/internal/model/price"
	"github.com/ananthakumaran/paisa/internal/service"
	"github.com/ananthakumaran/paisa/internal/utils"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const indiaTaxContractConfig = `journal_path: main.ledger
db_path: paisa.db
default_currency: EUR
time_zone: UTC
tax_regime: india
financial_year_starting_month: 1
commodities:
  - name: EQUITY
    type: stock
    price:
      provider: com-yahoo
      code: EQUITY
    tax_category: equity
    harvest: 365
schedule_al:
  - code: bank
    accounts:
      - "Assets:Bank"
`

const germanyTaxContractConfig = `journal_path: main.ledger
db_path: paisa.db
default_currency: EUR
time_zone: UTC
tax_regime: germany
financial_year_starting_month: 1
germany_tax:
  annual_allowance: 100
  capital_income_tax_rate: 0.25
  solidarity_surcharge_rate: 0.055
  church_tax_rate: 0
commodities:
  - name: VWCE
    type: mutualfund
    price:
      provider: com-yahoo
      code: VWCE
    germany_partial_exemption_rate: 0.3
`

const germanyStockTaxContractConfig = `journal_path: main.ledger
db_path: paisa.db
default_currency: EUR
time_zone: UTC
tax_regime: germany
financial_year_starting_month: 1
commodities:
  - name: ACM
    type: stock
    price:
      provider: com-yahoo
      code: ACM
`

func openTaxContractDB(t *testing.T) *gorm.DB {
	t.Helper()

	name := strings.NewReplacer("/", "_", "\\", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&posting.Posting{},
		&price.Price{},
		&cache.Cache{},
	))

	service.ClearPriceCache()
	accounting.ClearCache()
	t.Cleanup(service.ClearPriceCache)
	t.Cleanup(accounting.ClearCache)

	return db
}

func taxContractRouter(t *testing.T, db *gorm.DB, content string) *ginEngine {
	t.Helper()

	require.NoError(t, config.LoadConfig([]byte(content), ""))
	store := config.NewStore()
	require.NoError(t, store.Load([]byte(content), ""))

	return &ginEngine{router: Build(Deps{DB: db, Config: store}, false)}
}

func taxContractPosting(id uint, transactionID string, date string, account string, commodity string, quantity string, amount string) posting.Posting {
	parsed, err := time.ParseInLocation("2006-01-02", date, time.UTC)
	if err != nil {
		panic(err)
	}

	return posting.Posting{
		ID:                   id,
		TransactionID:        transactionID,
		Date:                 parsed,
		Payee:                "Fixture Payee",
		Account:              account,
		Commodity:            commodity,
		Quantity:             decimal.RequireFromString(quantity),
		Amount:               decimal.RequireFromString(amount),
		Status:               "cleared",
		TransactionBeginLine: uint64(id),
		TransactionEndLine:   uint64(id) + 1,
		FileName:             "fixture.ledger",
	}
}

func requireFixture(t *testing.T, path string, body string) {
	t.Helper()

	expected, err := os.ReadFile(path)
	require.NoError(t, err)
	require.JSONEq(t, string(expected), body)
}

// TestHarvestJSONContract pins the wire format of GET /api/harvest, including
// the harvest breakdown dates and the tax numbers.
func TestHarvestJSONContract(t *testing.T) {
	utils.SetNow("2026-11-22")
	t.Cleanup(utils.ResetNow)

	db := openTaxContractDB(t)
	require.NoError(t, db.Create(&[]posting.Posting{
		taxContractPosting(1, "buy", "2024-01-10", "Assets:Equity:EQUITY", "EQUITY", "10", "1000"),
	}).Error)
	require.NoError(t, db.Create(&price.Price{
		Date:          time.Date(2026, 11, 22, 0, 0, 0, 0, time.UTC),
		CommodityType: config.Stock,
		CommodityName: "EQUITY",
		CommodityID:   "EQUITY",
		Value:         decimal.RequireFromString("150"),
	}).Error)

	engine := taxContractRouter(t, db, indiaTaxContractConfig)

	rec := engine.get(t, "/api/harvest")
	require.Equal(t, http.StatusOK, rec.Code)

	var body HarvestResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))

	requireFixture(t, "testdata/api/harvest.json", rec.Body.String())
}

// TestCapitalGainsJSONContract pins the wire format of GET /api/capital_gains,
// including the FIFO posting pair dates and the postings nested in them.
func TestCapitalGainsJSONContract(t *testing.T) {
	utils.SetNow("2026-11-22")
	t.Cleanup(utils.ResetNow)

	db := openTaxContractDB(t)
	require.NoError(t, db.Create(&[]posting.Posting{
		taxContractPosting(1, "buy", "2024-05-01", "Assets:Equity:EQUITY", "EQUITY", "10", "1000"),
		taxContractPosting(2, "sell", "2025-06-01", "Assets:Equity:EQUITY", "EQUITY", "-4", "-600"),
	}).Error)

	engine := taxContractRouter(t, db, indiaTaxContractConfig)

	rec := engine.get(t, "/api/capital_gains")
	require.Equal(t, http.StatusOK, rec.Code)

	var body CapitalGainsResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))

	requireFixture(t, "testdata/api/capital_gains.json", rec.Body.String())
}

// TestCapitalGainsOrphanSellJSONContract pins the empty posting pair list of a
// sell without a matching lot (for example a transfer-out). The generated type
// declares an array, so the wire value must be [] and never null.
func TestCapitalGainsOrphanSellJSONContract(t *testing.T) {
	utils.SetNow("2026-11-22")
	t.Cleanup(utils.ResetNow)

	db := openTaxContractDB(t)
	require.NoError(t, db.Create(&[]posting.Posting{
		taxContractPosting(1, "orphan-sell", "2025-06-01", "Assets:Equity:EQUITY", "EQUITY", "-4", "-600"),
	}).Error)

	engine := taxContractRouter(t, db, indiaTaxContractConfig)

	rec := engine.get(t, "/api/capital_gains")
	require.Equal(t, http.StatusOK, rec.Code)

	var body CapitalGainsResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))

	requireFixture(t, "testdata/api/capital_gains_orphan_sell.json", rec.Body.String())
}

// TestGermanyTaxEmptyDBJSONContract exercises the Germany branch of
// GET /api/capital_income_tax without postings, so the unsupported-regime
// short-circuit is not the only path under test.
func TestGermanyTaxEmptyDBJSONContract(t *testing.T) {
	utils.SetNow("2026-11-22")
	t.Cleanup(utils.ResetNow)

	db := openTaxContractDB(t)
	engine := taxContractRouter(t, db, germanyTaxContractConfig)

	rec := engine.get(t, "/api/capital_income_tax")
	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"tax_years":{}}`, rec.Body.String())

	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assertNoNull(t, body, "/api/capital_income_tax")
}

// TestGermanyTaxJSONContract pins the wire format of GET
// /api/capital_income_tax, including the nested posting pairs and the
// optional partial exemption rate.
func TestGermanyTaxJSONContract(t *testing.T) {
	utils.SetNow("2026-11-22")
	t.Cleanup(utils.ResetNow)

	db := openTaxContractDB(t)
	require.NoError(t, db.Create(&[]posting.Posting{
		taxContractPosting(1, "buy", "2024-01-10", "Assets:Equity:VWCE", "VWCE", "10", "1000"),
		taxContractPosting(2, "sell", "2024-02-15", "Assets:Equity:VWCE", "VWCE", "-4", "-520"),
		taxContractPosting(3, "dividend", "2024-03-01", "Income:Dividend:VWCE", "EUR", "0", "-45.67"),
		taxContractPosting(4, "withholding", "2024-03-01", "Expenses:Broker:Taxes", "EUR", "0", "30"),
	}).Error)

	engine := taxContractRouter(t, db, germanyTaxContractConfig)

	rec := engine.get(t, "/api/capital_income_tax")
	require.Equal(t, http.StatusOK, rec.Code)

	var body GermanyTaxResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))

	requireFixture(t, "testdata/api/capital_income_tax.json", rec.Body.String())
}

// TestGermanyTaxOmitsMissingPartialExemptionRate pins the optional-pointer
// contract: a stock without a partial exemption rate must not emit a null key.
func TestGermanyTaxOmitsMissingPartialExemptionRate(t *testing.T) {
	utils.SetNow("2026-11-22")
	t.Cleanup(utils.ResetNow)

	db := openTaxContractDB(t)
	require.NoError(t, db.Create(&[]posting.Posting{
		taxContractPosting(1, "buy", "2024-01-10", "Assets:Equity:ACM", "ACM", "10", "1000"),
		taxContractPosting(2, "sell", "2024-02-15", "Assets:Equity:ACM", "ACM", "-4", "-520"),
	}).Error)

	engine := taxContractRouter(t, db, germanyStockTaxContractConfig)

	rec := engine.get(t, "/api/capital_income_tax")
	require.Equal(t, http.StatusOK, rec.Code)
	require.NotContains(t, rec.Body.String(), "partial_exemption_rate")

	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assertNoNull(t, body, "/api/capital_income_tax")
}

// TestTaxOverviewJSONContract pins the wire format of GET /api/tax_overview,
// including the section/category structure, the income tax payments and the
// capital income summary.
func TestTaxOverviewJSONContract(t *testing.T) {
	utils.SetNow("2026-10-27")
	t.Cleanup(utils.ResetNow)

	db := openTaxContractDB(t)
	require.NoError(t, db.Create(&[]posting.Posting{
		{
			ID:                   1,
			TransactionID:        "chair",
			Date:                 time.Date(2025, 9, 9, 0, 0, 0, 0, time.UTC),
			Payee:                "Chair Shop",
			Account:              "Expenses:Household:Goods",
			Commodity:            "EUR",
			Quantity:             decimal.RequireFromString("699"),
			Amount:               decimal.RequireFromString("699"),
			TaxCategory:          "work_equipment",
			TransactionBeginLine: 10,
			TransactionEndLine:   12,
			FileName:             "fixture.ledger",
			Note:                 "desk chair",
		},
		{
			ID:                   2,
			TransactionID:        "tax",
			Date:                 time.Date(2025, 6, 10, 0, 0, 0, 0, time.UTC),
			Payee:                "FINANZAMT",
			Account:              "Expenses:Tax:IncomeTax",
			Commodity:            "EUR",
			Quantity:             decimal.RequireFromString("110"),
			Amount:               decimal.RequireFromString("110"),
			TransactionBeginLine: 20,
			TransactionEndLine:   22,
			FileName:             "fixture.ledger",
		},
		{
			ID:                   3,
			TransactionID:        "dividend",
			Date:                 time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
			Payee:                "Broker",
			Account:              "Income:Dividend:VWCE",
			Commodity:            "EUR",
			Quantity:             decimal.RequireFromString("0"),
			Amount:               decimal.RequireFromString("-118.06"),
			TransactionBeginLine: 30,
			TransactionEndLine:   32,
			FileName:             "fixture.ledger",
		},
		{
			ID:                   4,
			TransactionID:        "withholding",
			Date:                 time.Date(2025, 6, 2, 0, 0, 0, 0, time.UTC),
			Payee:                "Broker",
			Account:              "Expenses:Broker:Taxes",
			Commodity:            "EUR",
			Quantity:             decimal.RequireFromString("0"),
			Amount:               decimal.RequireFromString("12"),
			TransactionBeginLine: 40,
			TransactionEndLine:   42,
			FileName:             "fixture.ledger",
		},
	}).Error)

	engine := taxContractRouter(t, db, germanyTaxContractConfig)

	rec := engine.get(t, "/api/tax_overview")
	require.Equal(t, http.StatusOK, rec.Code)

	var body TaxOverview
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))

	requireFixture(t, "testdata/api/tax_overview.json", rec.Body.String())
}

// TestScheduleALJSONContract pins the wire format of GET /api/schedule_al,
// including the financial-year key, the statement date and the fixed section
// catalog.
func TestScheduleALJSONContract(t *testing.T) {
	utils.SetNow("2026-11-22")
	t.Cleanup(utils.ResetNow)

	db := openTaxContractDB(t)
	require.NoError(t, db.Create(&[]posting.Posting{
		taxContractPosting(1, "bank", "2026-01-10", "Assets:Bank", "EUR", "5000", "5000"),
		taxContractPosting(2, "equity", "2026-05-01", "Assets:Equity:EQUITY", "EUR", "1000", "1000"),
	}).Error)

	engine := taxContractRouter(t, db, indiaTaxContractConfig)

	rec := engine.get(t, "/api/schedule_al")
	require.Equal(t, http.StatusOK, rec.Code)

	var body ScheduleALResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))

	requireFixture(t, "testdata/api/schedule_al.json", rec.Body.String())
}

// TestTaxResponsesHaveNoNullOnEmptyDB pins the null-vs-empty contract of the
// tax endpoints. The generated TypeScript types declare arrays and maps, so an
// empty journal must not produce JSON null.
func TestTaxResponsesHaveNoNullOnEmptyDB(t *testing.T) {
	utils.SetNow("2026-11-22")
	t.Cleanup(utils.ResetNow)

	db := openTaxContractDB(t)
	engine := taxContractRouter(t, db, indiaTaxContractConfig)

	for _, path := range []string{
		"/api/harvest",
		"/api/capital_gains",
		"/api/capital_income_tax",
		"/api/tax_overview",
		"/api/schedule_al",
	} {
		rec := engine.get(t, path)
		require.Equal(t, http.StatusOK, rec.Code, path)

		var body map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), path)
		assertNoNull(t, body, path)
	}
}
