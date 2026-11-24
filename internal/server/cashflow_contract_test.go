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

const cashflowContractConfig = `journal_path: main.ledger
db_path: paisa.db
default_currency: EUR
time_zone: UTC
financial_year_starting_month: 1
`

func openCashflowContractDB(t *testing.T) *gorm.DB {
	t.Helper()

	name := strings.NewReplacer("/", "_", "\\", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&posting.Posting{},
		&price.Price{},
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

func loadCashflowContractConfig(t *testing.T) {
	t.Helper()

	require.NoError(t, config.LoadConfig([]byte(cashflowContractConfig), ""))
	t.Cleanup(func() {
		_ = config.LoadConfig([]byte(cashflowContractConfig), "")
	})
}

func cashflowContractRouter(t *testing.T, db *gorm.DB) *ginEngine {
	t.Helper()

	loadCashflowContractConfig(t)
	store := config.NewStore()
	require.NoError(t, store.Load([]byte(cashflowContractConfig), ""))

	return &ginEngine{router: Build(Deps{DB: db, Config: store}, false)}
}

// cashflowContractPostings is a stable fixture with one month of income,
// expenses, tax, investment and liability postings.
func cashflowContractPostings() []posting.Posting {
	date := time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)

	mk := func(id uint, transactionID string, account string, amount int64) posting.Posting {
		return posting.Posting{
			ID:            id,
			TransactionID: transactionID,
			Date:          date,
			Payee:         "Fixture Payee",
			Account:       account,
			Commodity:     "EUR",
			Quantity:      decimal.NewFromInt(amount),
			Amount:        decimal.NewFromInt(amount),
			Status:        "cleared",
			FileName:      "fixture.ledger",
		}
	}

	return []posting.Posting{
		mk(1, "fixture-salary", "Income:Salary", -1000),
		mk(2, "fixture-salary", "Assets:Checking:Bank", 1000),
		mk(3, "fixture-food", "Expenses:Food", 200),
		mk(4, "fixture-food", "Assets:Checking:Bank", -200),
		mk(5, "fixture-tax", "Expenses:Tax:IncomeTax", 100),
		mk(6, "fixture-tax", "Assets:Checking:Bank", -100),
		mk(7, "fixture-buy", "Assets:Broker", 300),
		mk(8, "fixture-buy", "Assets:Checking:Bank", -300),
		mk(9, "fixture-loan", "Liabilities:Loan", -50),
		mk(10, "fixture-loan", "Assets:Checking:Bank", 50),
	}
}

// TestCashFlowJSONContract pins the wire format of GET /api/cash_flow. The
// fixture is checked in so that renaming a JSON field or changing a date from a
// string to an object cannot silently break the generated TypeScript types.
func TestCashFlowJSONContract(t *testing.T) {
	utils.SetNow("2026-11-22")
	t.Cleanup(utils.ResetNow)

	db := openCashflowContractDB(t)
	require.NoError(t, db.Create(cashflowContractPostings()).Error)
	engine := cashflowContractRouter(t, db)

	rec := engine.get(t, "/api/cash_flow")
	require.Equal(t, http.StatusOK, rec.Code)

	var body CashFlowResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))

	expected, err := os.ReadFile("testdata/api/cash_flow.json")
	require.NoError(t, err)
	require.JSONEq(t, string(expected), rec.Body.String())
}

// TestIncomeStatementJSONContract pins the wire format of GET
// /api/income_statement, including the nested map of financial years.
func TestIncomeStatementJSONContract(t *testing.T) {
	utils.SetNow("2026-11-22")
	t.Cleanup(utils.ResetNow)

	db := openCashflowContractDB(t)
	require.NoError(t, db.Create(cashflowContractPostings()).Error)
	engine := cashflowContractRouter(t, db)

	rec := engine.get(t, "/api/income_statement")
	require.Equal(t, http.StatusOK, rec.Code)

	var body IncomeStatementResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))

	expected, err := os.ReadFile("testdata/api/income_statement.json")
	require.NoError(t, err)
	require.JSONEq(t, string(expected), rec.Body.String())
}

// TestCashflowResponsesHaveNoNullOnEmptyDB pins the null-vs-empty contract of
// the cashflow endpoints. The generated TypeScript types declare arrays and
// maps, so an empty journal must not produce JSON null.
func TestCashflowResponsesHaveNoNullOnEmptyDB(t *testing.T) {
	utils.SetNow("2026-11-22")
	t.Cleanup(utils.ResetNow)

	db := openCashflowContractDB(t)
	engine := cashflowContractRouter(t, db)

	for _, path := range []string{
		"/api/income",
		"/api/expense",
		"/api/budget",
		"/api/cash_flow",
		"/api/income_statement",
		"/api/recurring",
	} {
		rec := engine.get(t, path)
		require.Equal(t, http.StatusOK, rec.Code, path)

		var body map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), path)
		assertNoNull(t, body, path)
	}
}
