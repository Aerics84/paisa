package server

import (
	"testing"
	"time"

	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/ananthakumaran/paisa/internal/query"
	"github.com/ananthakumaran/paisa/internal/utils"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestComputeCashFlowClassifiesPostings(t *testing.T) {
	loadMonthlyReviewConfig(t)
	utils.SetNow("2022-03-15")
	t.Cleanup(func() { utils.SetNow(time.Now().Format("2006-01-02")) })
	db := openMonthlyReviewTestDB(t)

	date := monthlyReviewTestDate("2022-01-15")
	rows := []posting.Posting{
		{TransactionID: "salary", Date: date, Account: "Income:Salary:Acme", Commodity: "EUR", Amount: decimal.NewFromInt(-100)},
		{TransactionID: "salary", Date: date, Account: "Assets:Checking:Bank", Commodity: "EUR", Amount: decimal.NewFromInt(100)},
		{TransactionID: "food", Date: date, Account: "Expenses:Food:Groceries", Commodity: "EUR", Amount: decimal.NewFromInt(30)},
		{TransactionID: "food", Date: date, Account: "Assets:Checking:Bank", Commodity: "EUR", Amount: decimal.NewFromInt(-30)},
		{TransactionID: "tax", Date: date, Account: "Expenses:Tax:Income", Commodity: "EUR", Amount: decimal.NewFromInt(10)},
		{TransactionID: "tax", Date: date, Account: "Assets:Checking:Bank", Commodity: "EUR", Amount: decimal.NewFromInt(-10)},
		{TransactionID: "buy", Date: date, Account: "Assets:Broker", Commodity: "EUR", Amount: decimal.NewFromInt(50)},
		{TransactionID: "buy", Date: date, Account: "Assets:Checking:Bank", Commodity: "EUR", Amount: decimal.NewFromInt(-50)},
		{TransactionID: "loan", Date: date, Account: "Liabilities:Loan", Commodity: "EUR", Amount: decimal.NewFromInt(-20)},
		{TransactionID: "loan", Date: date, Account: "Assets:Checking:Bank", Commodity: "EUR", Amount: decimal.NewFromInt(20)},
	}
	require.NoError(t, db.Create(&rows).Error)

	flows := computeCashFlow(db, query.Init(db), decimal.Zero)
	require.NotEmpty(t, flows)

	jan := flows[0]
	require.Equal(t, "2022-01-01", jan.Date.Format("2006-01-02"))
	requireDecimalEqual(t, "100", jan.Income)
	requireDecimalEqual(t, "30", jan.Expenses)
	requireDecimalEqual(t, "10", jan.Tax)
	requireDecimalEqual(t, "50", jan.Investment)
	requireDecimalEqual(t, "20", jan.Liabilities)
	requireDecimalEqual(t, "30", jan.Checking)
	requireDecimalEqual(t, "30", jan.Balance)
}

func TestComputeCashFlowKeepsTaxOutOfExpenses(t *testing.T) {
	loadMonthlyReviewConfig(t)
	utils.SetNow("2022-03-15")
	t.Cleanup(func() { utils.SetNow(time.Now().Format("2006-01-02")) })
	db := openMonthlyReviewTestDB(t)

	date := monthlyReviewTestDate("2022-01-15")
	rows := []posting.Posting{
		{TransactionID: "salary", Date: date, Account: "Income:Salary:Acme", Commodity: "EUR", Amount: decimal.NewFromInt(-5000)},
		{TransactionID: "salary", Date: date, Account: "Assets:Checking:Bank", Commodity: "EUR", Amount: decimal.NewFromInt(5000)},
		{TransactionID: "food", Date: date, Account: "Expenses:Food:Groceries", Commodity: "EUR", Amount: decimal.NewFromInt(2000)},
		{TransactionID: "food", Date: date, Account: "Assets:Checking:Bank", Commodity: "EUR", Amount: decimal.NewFromInt(-2000)},
		{TransactionID: "tax", Date: date, Account: "Expenses:Tax:IncomeTax", Commodity: "EUR", Amount: decimal.NewFromInt(500)},
		{TransactionID: "tax", Date: date, Account: "Assets:Checking:Bank", Commodity: "EUR", Amount: decimal.NewFromInt(-500)},
		{TransactionID: "buy", Date: date, Account: "Assets:Broker", Commodity: "EUR", Amount: decimal.NewFromInt(1000)},
		{TransactionID: "buy", Date: date, Account: "Assets:Checking:Bank", Commodity: "EUR", Amount: decimal.NewFromInt(-1000)},
	}
	require.NoError(t, db.Create(&rows).Error)

	flows := computeCashFlow(db, query.Init(db), decimal.Zero)
	require.NotEmpty(t, flows)

	jan := flows[0]
	requireDecimalEqual(t, "5000", jan.Income)
	requireDecimalEqual(t, "2000", jan.Expenses) // tax must not be included here
	requireDecimalEqual(t, "500", jan.Tax)
	requireDecimalEqual(t, "1000", jan.Investment)
}
