package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/ananthakumaran/paisa/internal/utils"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func loadTaxOverviewConfig(t *testing.T) {
	t.Helper()
	require.NoError(t, config.LoadConfig([]byte(`
journal_path: test.ledger
db_path: test.db
tax_regime: germany
default_currency: EUR
time_zone: Europe/Berlin
`), ""))
}

func openTaxOverviewTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	name := strings.NewReplacer("/", "_", "\\", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&posting.Posting{}))
	return db
}

func TestGetTaxOverviewBuildsSelectedYear(t *testing.T) {
	utils.SetNow("2026-10-27")
	t.Cleanup(func() { utils.SetNow(time.Now().Format("2006-01-02")) })
	loadTaxOverviewConfig(t)
	db := openTaxOverviewTestDB(t)

	date := func(value string) time.Time {
		parsed, err := time.Parse("2006-01-02", value)
		require.NoError(t, err)
		return parsed
	}
	rows := []posting.Posting{
		{TransactionID: "health", Date: date("2025-05-01"), Account: "Expenses:Health", Amount: decimal.RequireFromString("100")},
		{TransactionID: "chair", Date: date("2025-09-09"), Account: "Expenses:Household:Goods", Amount: decimal.RequireFromString("699"), TaxCategory: "work_equipment"},
		{TransactionID: "untagged", Date: date("2025-07-01"), Account: "Expenses:Household:Goods", Amount: decimal.RequireFromString("50")},
		{TransactionID: "dividend", Date: date("2025-06-01"), Account: "Income:Dividend:VWCE", Amount: decimal.RequireFromString("-118.06")},
		{TransactionID: "interest", Date: date("2024-06-01"), Account: "Income:Interest:Bank", Amount: decimal.RequireFromString("-10")},
		{TransactionID: "next-year", Date: date("2026-05-01"), Account: "Expenses:Health", Amount: decimal.RequireFromString("30")},
	}
	require.NoError(t, db.Create(&rows).Error)

	overview, err := GetTaxOverview(db, "2025")
	require.NoError(t, err)

	assert.Equal(t, "2025", overview.SelectedYear)
	assert.Contains(t, overview.Years, "2024")
	assert.Contains(t, overview.Years, "2025")
	assert.Contains(t, overview.Years, "2026")
	assert.True(t, overview.Summary.ExpenseCandidates.Equal(decimal.RequireFromString("799")))
	assert.Equal(t, 2, overview.Summary.MatchedExpensePostings)
	assert.Equal(t, 1, overview.Coverage.UnmatchedPostingCount)
	assert.Equal(t, 1, overview.Summary.DiagnosticCount)
	assert.True(t, overview.Summary.CapitalIncomeBooked.Equal(decimal.RequireFromString("118.06")))
	assert.True(t, overview.Summary.CapitalIncomeCalculated.Equal(decimal.Zero))
}

func TestTaxOverviewJSONUsesNumbersForDecimals(t *testing.T) {
	utils.SetNow("2026-10-27")
	t.Cleanup(func() { utils.SetNow(time.Now().Format("2006-01-02")) })
	loadTaxOverviewConfig(t)
	db := openTaxOverviewTestDB(t)
	require.NoError(t, db.Create(&posting.Posting{
		TransactionID: "health", Date: time.Date(2025, 5, 1, 0, 0, 0, 0, time.UTC),
		Account: "Expenses:Health", Amount: decimal.RequireFromString("100"),
	}).Error)

	overview, err := GetTaxOverview(db, "2025")
	require.NoError(t, err)
	body, err := json.Marshal(overview)
	require.NoError(t, err)
	assert.Contains(t, string(body), `"expense_candidates":100`)
	assert.NotContains(t, string(body), `"expense_candidates":"100"`)
	assert.Contains(t, string(body), `"capital_income_calculated":0`)
}

func TestServerDecimalJSONIsNumber(t *testing.T) {
	body, err := json.Marshal(struct {
		Amount decimal.Decimal `json:"amount"`
	}{Amount: decimal.RequireFromString("1.23")})
	require.NoError(t, err)
	assert.JSONEq(t, `{"amount":1.23}`, string(body))
}

func TestGetTaxOverviewDefaultsToPreviousYear(t *testing.T) {
	utils.SetNow("2026-10-27")
	t.Cleanup(func() { utils.SetNow(time.Now().Format("2006-01-02")) })
	loadTaxOverviewConfig(t)
	db := openTaxOverviewTestDB(t)

	overview, err := GetTaxOverview(db, "")
	require.NoError(t, err)
	assert.Equal(t, "2025", overview.SelectedYear)
}

func TestTaxOverviewYearValidation(t *testing.T) {
	now := time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)
	for _, requested := range []string{"", "2025", "2019"} {
		got, err := taxOverviewYear(requested, now)
		require.NoError(t, err, requested)
		if requested == "" {
			assert.Equal(t, "2025", got)
		} else {
			assert.Equal(t, requested, got)
		}
	}
	for _, requested := range []string{"abc", "25", "20255", "20x5"} {
		_, err := taxOverviewYear(requested, now)
		assert.Error(t, err, requested)
	}
}

func TestGetTaxOverviewAllowsValidYearWithoutPostings(t *testing.T) {
	loadTaxOverviewConfig(t)
	db := openTaxOverviewTestDB(t)
	overview, err := GetTaxOverview(db, "2019")
	require.NoError(t, err)
	assert.Equal(t, "2019", overview.SelectedYear)
	assert.NotEmpty(t, overview.Sections)
	assert.Equal(t, 0, overview.Coverage.UnmatchedPostingCount)
}

func TestTaxOverviewRouteRejectsInvalidYear(t *testing.T) {
	loadTaxOverviewConfig(t)
	db := openTaxOverviewTestDB(t)
	router := Build(db, false)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/tax_overview?year=abc", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
