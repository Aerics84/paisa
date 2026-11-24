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

const goalsContractConfig = `journal_path: main.ledger
db_path: paisa.db
default_currency: EUR
time_zone: UTC
financial_year_starting_month: 1
goals:
  retirement:
    - name: Fire
      icon: retirement
      swr: 4
      yearly_expenses: 40000
      savings:
        - "Assets:Goal:Retirement"
      priority: 1
  savings:
    - name: Vacation
      icon: vacation
      target: 10000
      target_date: "2027-06-30"
      rate: 0.05
      payment_per_period: 500
      accounts:
        - "Assets:Goal:Vacation"
      priority: 2
`

func openGoalsContractDB(t *testing.T) *gorm.DB {
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

func goalsContractRouter(t *testing.T, db *gorm.DB) *ginEngine {
	t.Helper()

	require.NoError(t, config.LoadConfig([]byte(goalsContractConfig), ""))
	store := config.NewStore()
	require.NoError(t, store.Load([]byte(goalsContractConfig), ""))

	return &ginEngine{router: Build(Deps{DB: db, Config: store}, false)}
}

// goalsContractPostings is a stable fixture with one balance per configured
// goal. The posting is before the current month so it also appears in the
// savings contribution history.
func goalsContractPostings() []posting.Posting {
	mk := func(id uint64, transactionID string, date string, account string, amount int64) posting.Posting {
		parsed, err := time.ParseInLocation("2006-01-02", date, time.UTC)
		if err != nil {
			panic(err)
		}

		return posting.Posting{
			ID:                   uint(id),
			TransactionID:        transactionID,
			Date:                 parsed,
			Payee:                "Fixture Payee",
			Account:              account,
			Commodity:            "EUR",
			Quantity:             decimal.NewFromInt(amount),
			Amount:               decimal.NewFromInt(amount),
			Status:               "cleared",
			TransactionBeginLine: id,
			TransactionEndLine:   id + 1,
			FileName:             "fixture.ledger",
		}
	}

	return []posting.Posting{
		mk(1, "fixture-retirement", "2026-10-31", "Assets:Goal:Retirement", 20000),
		mk(2, "fixture-vacation", "2026-10-31", "Assets:Goal:Vacation", 4000),
	}
}

// TestGoalsJSONContract pins the wire format of GET /api/goals, including the
// nested goal intelligence.
func TestGoalsJSONContract(t *testing.T) {
	utils.SetNow("2026-11-22")
	t.Cleanup(utils.ResetNow)

	db := openGoalsContractDB(t)
	require.NoError(t, db.Create(goalsContractPostings()).Error)
	engine := goalsContractRouter(t, db)

	rec := engine.get(t, "/api/goals")
	require.Equal(t, http.StatusOK, rec.Code)

	var body GoalsResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))

	expected, err := os.ReadFile("testdata/api/goals.json")
	require.NoError(t, err)
	require.JSONEq(t, string(expected), rec.Body.String())
}

// TestRetirementGoalDetailJSONContract pins the nested retirement projection:
// savings timeline points, postings, balance breakdowns and intelligence.
func TestRetirementGoalDetailJSONContract(t *testing.T) {
	utils.SetNow("2026-11-22")
	t.Cleanup(utils.ResetNow)

	db := openGoalsContractDB(t)
	require.NoError(t, db.Create(goalsContractPostings()).Error)
	engine := goalsContractRouter(t, db)

	rec := engine.get(t, "/api/goals/retirement/Fire")
	require.Equal(t, http.StatusOK, rec.Code)
	require.NotEqual(t, "{}", strings.TrimSpace(rec.Body.String()))

	expected, err := os.ReadFile("testdata/api/goal_retirement.json")
	require.NoError(t, err)
	require.JSONEq(t, string(expected), rec.Body.String())
}

// TestSavingsGoalDetailJSONContract pins the nested savings projection,
// including the date-only targetDate and the contribution history.
func TestSavingsGoalDetailJSONContract(t *testing.T) {
	utils.SetNow("2026-11-22")
	t.Cleanup(utils.ResetNow)

	db := openGoalsContractDB(t)
	require.NoError(t, db.Create(goalsContractPostings()).Error)
	engine := goalsContractRouter(t, db)

	rec := engine.get(t, "/api/goals/savings/Vacation")
	require.Equal(t, http.StatusOK, rec.Code)
	require.NotEqual(t, "{}", strings.TrimSpace(rec.Body.String()))

	expected, err := os.ReadFile("testdata/api/goal_savings.json")
	require.NoError(t, err)
	require.JSONEq(t, string(expected), rec.Body.String())
}

// TestGoalResponsesHaveNoNullOnEmptyDB pins the null-vs-empty contract of the
// goals endpoints. The generated TypeScript types declare arrays and maps, so
// an empty journal must not produce JSON null.
func TestGoalResponsesHaveNoNullOnEmptyDB(t *testing.T) {
	utils.SetNow("2026-11-22")
	t.Cleanup(utils.ResetNow)

	db := openGoalsContractDB(t)
	engine := goalsContractRouter(t, db)

	for _, path := range []string{
		"/api/goals",
		"/api/goals/retirement/Fire",
		"/api/goals/savings/Vacation",
	} {
		rec := engine.get(t, path)
		require.Equal(t, http.StatusOK, rec.Code, path)

		var body map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), path)
		assertNoNull(t, body, path)
	}
}
