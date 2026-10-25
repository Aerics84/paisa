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
	"github.com/ananthakumaran/paisa/internal/model/price"

	"github.com/ananthakumaran/paisa/internal/service"
	"github.com/ananthakumaran/paisa/internal/utils"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func monthlyReviewTestDate(date string) time.Time {
	parsed, err := time.ParseInLocation("2006-01-02", date, config.TimeZone())
	if err != nil {
		panic(err)
	}
	return parsed
}

func setMonthlyReviewNow(t *testing.T, date string) {
	t.Helper()
	utils.SetNow(date)
}

func openMonthlyReviewTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	loadMonthlyReviewConfig(t)

	name := strings.NewReplacer("/", "_", "\\", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&posting.Posting{}, &price.Price{}))
	service.ClearPriceCache()
	t.Cleanup(service.ClearPriceCache)
	return db
}

func addMonthlyReviewPosting(t *testing.T, db *gorm.DB, transactionID string, date string, account string, commodity string, quantity string, amount string) {
	t.Helper()

	require.NoError(t, db.Create(&posting.Posting{
		TransactionID: transactionID,
		Date:          monthlyReviewTestDate(date),
		Payee:         transactionID,
		Account:       account,
		Commodity:     commodity,
		Quantity:      decimal.RequireFromString(quantity),
		Amount:        decimal.RequireFromString(amount),
	}).Error)
}

func addMonthlyReviewCurrencyPosting(t *testing.T, db *gorm.DB, transactionID string, date string, account string, amount string) {
	t.Helper()
	addMonthlyReviewPosting(t, db, transactionID, date, account, "EUR", amount, amount)
}

func addMonthlyReviewSalary(t *testing.T, db *gorm.DB, transactionID string, date string, amount string) {
	t.Helper()
	addMonthlyReviewPosting(t, db, transactionID, date, "Income:Salary:Acme", "EUR", "-"+amount, "-"+amount)
	addMonthlyReviewCurrencyPosting(t, db, transactionID, date, "Assets:Checking:Bank", amount)
}

func addMonthlyReviewExpense(t *testing.T, db *gorm.DB, transactionID string, date string, amount string) {
	t.Helper()
	addMonthlyReviewCurrencyPosting(t, db, transactionID, date, "Expenses:Food:Groceries", amount)
	addMonthlyReviewCurrencyPosting(t, db, transactionID, date, "Assets:Checking:Bank", "-"+amount)
}

func addMonthlyReviewPrice(t *testing.T, db *gorm.DB, date string, commodity string, value string) {
	t.Helper()
	require.NoError(t, db.Create(&price.Price{
		Date:          monthlyReviewTestDate(date),
		CommodityType: config.Stock,
		CommodityID:   commodity,
		CommodityName: commodity,
		Value:         decimal.RequireFromString(value),
	}).Error)
}

func addWeeklyMonthlyReviewPrices(t *testing.T, db *gorm.DB, commodity string, dates []string, value string) {
	t.Helper()
	for _, date := range dates {
		addMonthlyReviewPrice(t, db, date, commodity, value)
	}
}

func TestGetMonthlyReviewUsesSelectedCalendarMonth(t *testing.T) {
	setMonthlyReviewNow(t, "2022-03-15")
	db := openMonthlyReviewTestDB(t)

	addMonthlyReviewSalary(t, db, "jan-salary", "2022-01-15", "1000")
	addMonthlyReviewExpense(t, db, "jan-expense", "2022-01-20", "200")
	addMonthlyReviewSalary(t, db, "feb-salary", "2022-02-15", "2000")
	addMonthlyReviewExpense(t, db, "feb-expense", "2022-02-20", "500")

	review := GetMonthlyReview(db, monthlyReviewTestDate("2022-02-01"))

	require.Equal(t, "2022-02", review.Month)
	require.Equal(t, "ready", review.ActivityState)
	require.True(t, review.PeriodComplete)
	requireDecimalEqual(t, "2000", review.NetIncomeAfterTax)
	requireDecimalEqual(t, "500", review.NetExpense)
	requireDecimalEqual(t, "1500", review.Surplus)
	require.True(t, review.HasIncomePostings)
	require.True(t, review.HasExpensePostings)

	require.Len(t, review.NetIncomeTrend, 3)
	require.Equal(t, "2021-12", review.NetIncomeTrend[0].Month)
	require.False(t, review.NetIncomeTrend[0].Available)
	require.Equal(t, "2022-01", review.NetIncomeTrend[1].Month)
	require.True(t, review.NetIncomeTrend[1].Available)
	requireDecimalEqual(t, "1000", review.NetIncomeTrend[1].Value)
	require.Equal(t, "2022-02", review.NetIncomeTrend[2].Month)
	require.True(t, review.NetIncomeTrend[2].Available)
	requireDecimalEqual(t, "2000", review.NetIncomeTrend[2].Value)
	require.False(t, review.NetIncomeTrend[2].IsMonthToDate)
}

func TestGetMonthlyReviewSubtractsTaxExactlyOnce(t *testing.T) {
	setMonthlyReviewNow(t, "2022-03-15")
	db := openMonthlyReviewTestDB(t)

	addMonthlyReviewSalary(t, db, "salary", "2022-02-01", "1000")
	addMonthlyReviewCurrencyPosting(t, db, "tax", "2022-02-05", "Expenses:Tax:Income", "100")
	addMonthlyReviewCurrencyPosting(t, db, "tax", "2022-02-05", "Assets:Checking:Bank", "-100")
	addMonthlyReviewExpense(t, db, "expense", "2022-02-06", "200")

	review := GetMonthlyReview(db, monthlyReviewTestDate("2022-02-01"))

	requireDecimalEqual(t, "100", review.NetTax)
	requireDecimalEqual(t, "900", review.NetIncomeAfterTax)
	requireDecimalEqual(t, "200", review.NetExpense)
	requireDecimalEqual(t, "700", review.Surplus)
}

func TestGetMonthlyReviewKeepsSurplusSeparateFromInvestmentAndCheckingMovements(t *testing.T) {
	setMonthlyReviewNow(t, "2022-03-15")
	db := openMonthlyReviewTestDB(t)

	addMonthlyReviewSalary(t, db, "salary", "2022-02-01", "1000")
	addMonthlyReviewExpense(t, db, "expense", "2022-02-06", "200")

	addMonthlyReviewPosting(t, db, "investment", "2022-02-10", "Assets:Equity:VWCE", "VWCE", "30", "300")
	addMonthlyReviewCurrencyPosting(t, db, "investment", "2022-02-10", "Assets:Checking:Bank", "-300")

	addMonthlyReviewCurrencyPosting(t, db, "transfer", "2022-02-12", "Assets:Checking:Savings", "-100")
	addMonthlyReviewCurrencyPosting(t, db, "transfer", "2022-02-12", "Assets:Checking:Bank", "100")

	review := GetMonthlyReview(db, monthlyReviewTestDate("2022-02-01"))

	requireDecimalEqual(t, "1000", review.NetIncomeAfterTax)
	requireDecimalEqual(t, "200", review.NetExpense)
	requireDecimalEqual(t, "800", review.Surplus)
	requireDecimalEqual(t, "300", review.InvestmentContributions)
}

func TestGetMonthlyReviewMarksCurrentMonthInProgress(t *testing.T) {
	setMonthlyReviewNow(t, "2022-02-15")
	db := openMonthlyReviewTestDB(t)

	addMonthlyReviewSalary(t, db, "salary", "2022-02-01", "500")
	addMonthlyReviewExpense(t, db, "expense", "2022-02-10", "100")
	addMonthlyReviewSalary(t, db, "future-salary", "2022-02-20", "999")

	review := GetMonthlyReview(db, monthlyReviewTestDate("2022-02-01"))

	require.False(t, review.PeriodComplete, "the current month is calendar-incomplete")
	require.Equal(t, "ready", review.ActivityState)
	requireDecimalEqual(t, "500", review.NetIncomeAfterTax) // future postings must not be counted
	requireDecimalEqual(t, "100", review.NetExpense)
	requireDecimalEqual(t, "400", review.Surplus)
	require.True(t, review.NetIncomeTrend[2].IsMonthToDate)
	requireDecimalEqual(t, "500", review.NetIncomeTrend[2].Value)

	january := GetMonthlyReview(db, monthlyReviewTestDate("2022-01-01"))
	require.True(t, january.PeriodComplete)
}

func TestGetMonthlyReviewShowsNoRecordedActivityWithoutClaimingCompleteness(t *testing.T) {
	setMonthlyReviewNow(t, "2022-04-15")
	db := openMonthlyReviewTestDB(t)

	addMonthlyReviewSalary(t, db, "jan-salary", "2022-01-15", "1000")

	review := GetMonthlyReview(db, monthlyReviewTestDate("2022-03-01"))

	require.Equal(t, "no_recorded_activity", review.ActivityState)
	require.True(t, review.PeriodComplete)
	requireDecimalEqual(t, "0", review.Surplus)
	require.Empty(t, review.FlowGraph.Nodes)
	require.Empty(t, review.FlowGraph.Links)
	require.Equal(t, "not_available", review.PerformanceState)
	require.Empty(t, review.BestPerformers)
	require.Empty(t, review.WeakestPerformers)
	require.False(t, review.HasIncomePostings)
	require.False(t, review.HasExpensePostings)
	require.Len(t, review.NetIncomeTrend, 3)
	require.Equal(t, "2022-01", review.NetIncomeTrend[0].Month)
	require.True(t, review.NetIncomeTrend[0].Available)
	require.False(t, review.NetIncomeTrend[1].Available)
	require.False(t, review.NetIncomeTrend[2].Available)
}

func TestGetMonthlyReviewReturnsFlowGraphFromActualPostings(t *testing.T) {
	setMonthlyReviewNow(t, "2022-03-15")
	db := openMonthlyReviewTestDB(t)

	addMonthlyReviewSalary(t, db, "salary", "2022-02-01", "1000")
	addMonthlyReviewExpense(t, db, "expense", "2022-02-06", "200")

	review := GetMonthlyReview(db, monthlyReviewTestDate("2022-02-01"))

	income, ok := findGraphLinkValue(review.FlowGraph, "Income", "Assets:Checking:Bank")
	require.True(t, ok)
	requireDecimalEqual(t, "1000", income)

	expense, ok := findGraphLinkValue(review.FlowGraph, "Assets:Checking:Bank", "Expenses")
	require.True(t, ok)
	requireDecimalEqual(t, "200", expense)

	leaf, ok := findGraphLinkValue(review.FlowGraph, "Expenses:Food", "Expenses:Food:Groceries")
	require.True(t, ok)
	requireDecimalEqual(t, "200", leaf)
}

func TestGetMonthlyReviewDoesNotRankPositionsWithoutPriceCoverage(t *testing.T) {
	setMonthlyReviewNow(t, "2022-03-15")
	db := openMonthlyReviewTestDB(t)

	addMonthlyReviewPosting(t, db, "buy", "2022-01-10", "Assets:Equity:VWCE", "VWCE", "10", "1000")
	addMonthlyReviewCurrencyPosting(t, db, "buy", "2022-01-10", "Assets:Checking:Bank", "-1000")

	review := GetMonthlyReview(db, monthlyReviewTestDate("2022-02-01"))

	require.Equal(t, "not_available", review.PerformanceState)
	require.Empty(t, review.BestPerformers)
	require.Empty(t, review.WeakestPerformers)
}

func TestGetMonthlyReviewRequiresTwoRankablePositions(t *testing.T) {
	setMonthlyReviewNow(t, "2022-03-15")
	db := openMonthlyReviewTestDB(t)

	addMonthlyReviewPosting(t, db, "buy", "2022-01-10", "Assets:Equity:VWCE", "VWCE", "10", "1000")
	addMonthlyReviewCurrencyPosting(t, db, "buy", "2022-01-10", "Assets:Checking:Bank", "-1000")
	addMonthlyReviewPrice(t, db, "2022-01-31", "VWCE", "100")
	addWeeklyMonthlyReviewPrices(t, db, "VWCE", []string{"2022-02-07", "2022-02-14", "2022-02-21", "2022-02-28"}, "110")
	service.ClearPriceCache()

	review := GetMonthlyReview(db, monthlyReviewTestDate("2022-02-01"))

	require.Equal(t, "not_available", review.PerformanceState, "a single position must not be presented as a best/weakest ranking")
	require.Empty(t, review.BestPerformers)
	require.Empty(t, review.WeakestPerformers)
}

func TestGetMonthlyReviewRanksFlowAdjustedPositions(t *testing.T) {
	setMonthlyReviewNow(t, "2022-03-15")
	db := openMonthlyReviewTestDB(t)

	addMonthlyReviewPosting(t, db, "buy-vwce", "2022-01-10", "Assets:Equity:VWCE", "VWCE", "10", "1000")
	addMonthlyReviewCurrencyPosting(t, db, "buy-vwce", "2022-01-10", "Assets:Checking:Bank", "-1000")
	addMonthlyReviewPosting(t, db, "buy-vti", "2022-01-10", "Assets:Equity:VTI", "VTI", "10", "1000")
	addMonthlyReviewCurrencyPosting(t, db, "buy-vti", "2022-01-10", "Assets:Checking:Bank", "-1000")

	addMonthlyReviewPrice(t, db, "2022-01-31", "VWCE", "100")
	addMonthlyReviewPrice(t, db, "2022-01-31", "VTI", "100")
	for _, priceDate := range []string{"2022-02-07", "2022-02-14", "2022-02-21", "2022-02-28"} {
		addMonthlyReviewPrice(t, db, priceDate, "VWCE", "110")
		addMonthlyReviewPrice(t, db, priceDate, "VTI", "90")
	}
	service.ClearPriceCache()

	review := GetMonthlyReview(db, monthlyReviewTestDate("2022-02-01"))

	t.Logf("performance state=%s best=%+v weakest=%+v asof=%s", review.PerformanceState, review.BestPerformers, review.WeakestPerformers, review.PerformanceAsOf)
	require.Equal(t, "ready", review.PerformanceState)
	require.NotEmpty(t, review.BestPerformers)
	require.NotEmpty(t, review.WeakestPerformers)
	require.Equal(t, "Assets:Equity:VWCE", review.BestPerformers[0].Account)
	requireDecimalEqual(t, "10", review.BestPerformers[0].TotalReturnPercent)
	require.Equal(t, "Assets:Equity:VTI", review.WeakestPerformers[0].Account)
	requireDecimalEqual(t, "-10", review.WeakestPerformers[0].TotalReturnPercent)
	require.True(t, review.PerformanceAsOf.Equal(monthlyReviewTestDate("2022-02-28")))
}

func TestGetMonthlyReviewAttributesDividendsOnlyWhenIdentifiable(t *testing.T) {
	setMonthlyReviewNow(t, "2022-03-15")
	db := openMonthlyReviewTestDB(t)

	addMonthlyReviewPosting(t, db, "buy-vwce", "2022-01-10", "Assets:Equity:VWCE", "VWCE", "10", "1000")
	addMonthlyReviewCurrencyPosting(t, db, "buy-vwce", "2022-01-10", "Assets:Checking:Bank", "-1000")
	addMonthlyReviewPosting(t, db, "buy-vti", "2022-01-10", "Assets:Equity:VTI", "VTI", "10", "1000")
	addMonthlyReviewCurrencyPosting(t, db, "buy-vti", "2022-01-10", "Assets:Checking:Bank", "-1000")

	for _, commodity := range []string{"VWCE", "VTI"} {
		addMonthlyReviewPrice(t, db, "2022-01-31", commodity, "100")
		addWeeklyMonthlyReviewPrices(t, db, commodity, []string{"2022-02-07", "2022-02-14", "2022-02-21", "2022-02-28"}, "100")
	}

	addMonthlyReviewCurrencyPosting(t, db, "dividend-vwce", "2022-02-15", "Income:Dividend:VWCE", "-5")
	addMonthlyReviewCurrencyPosting(t, db, "dividend-vwce", "2022-02-15", "Assets:Checking:Bank", "5")
	addMonthlyReviewCurrencyPosting(t, db, "dividend-unknown", "2022-02-15", "Income:Dividend:UNKNOWN", "-7")
	addMonthlyReviewCurrencyPosting(t, db, "dividend-unknown", "2022-02-15", "Assets:Checking:Bank", "7")
	service.ClearPriceCache()

	review := GetMonthlyReview(db, monthlyReviewTestDate("2022-02-01"))

	requireDecimalEqual(t, "12", review.DividendIncome)
	require.Equal(t, "ready", review.PerformanceState)
	require.Equal(t, "Assets:Equity:VWCE", review.BestPerformers[0].Account)
	requireDecimalEqual(t, "5", review.BestPerformers[0].DividendIncome)
	requireDecimalEqual(t, "0.5", review.BestPerformers[0].TotalReturnPercent)

	var vti *MonthlyPositionPerformance
	for index := range review.WeakestPerformers {
		if review.WeakestPerformers[index].Account == "Assets:Equity:VTI" {
			vti = &review.WeakestPerformers[index]
		}
	}
	require.NotNil(t, vti)
	requireDecimalEqual(t, "0", vti.DividendIncome, "an unidentifiable dividend must not be attributed to a position")
}

func TestGetMonthlyReviewExcludesQuantityOnlyStockSplit(t *testing.T) {
	setMonthlyReviewNow(t, "2022-03-15")
	db := openMonthlyReviewTestDB(t)

	addMonthlyReviewPosting(t, db, "buy", "2022-01-10", "Assets:Equity:VWCE", "VWCE", "10", "1000")
	addMonthlyReviewCurrencyPosting(t, db, "buy", "2022-01-10", "Assets:Checking:Bank", "-1000")
	addMonthlyReviewPosting(t, db, "split", "2022-02-15", "Assets:Equity:VWCE", "VWCE", "10", "0")
	addMonthlyReviewPosting(t, db, "split", "2022-02-15", "Assets:Equity:VWCE", "VWCE", "-1", "0")

	addMonthlyReviewPrice(t, db, "2022-01-31", "VWCE", "100")
	addWeeklyMonthlyReviewPrices(t, db, "VWCE", []string{"2022-02-07", "2022-02-14", "2022-02-21", "2022-02-28"}, "110")
	service.ClearPriceCache()

	review := GetMonthlyReview(db, monthlyReviewTestDate("2022-02-01"))

	require.Equal(t, "not_available", review.PerformanceState, "a split must not be ranked as a 900% return")
	require.Empty(t, review.BestPerformers)
}

func TestMonthlyDashboardRouteRejectsInvalidMonth(t *testing.T) {
	setMonthlyReviewNow(t, "2022-03-15")
	db := openMonthlyReviewTestDB(t)
	router := Build(db, false)

	for _, month := range []string{"", "2022", "2022-13", "abc", "2022-1"} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/dashboard/monthly?month="+month, nil))
		require.Equal(t, http.StatusBadRequest, recorder.Code, "month %q must be rejected", month)
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/dashboard/monthly?month=2022-02", nil))
	require.Equal(t, http.StatusOK, recorder.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, "2022-02", body["month"])
}
