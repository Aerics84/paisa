package server

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/ananthakumaran/paisa/internal/utils"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func configureInvestmentTest(t *testing.T) {
	t.Helper()
	root := t.TempDir()
	content, err := json.Marshal(map[string]interface{}{
		"journal_path":                  filepath.Join(root, "main.ledger"),
		"db_path":                       filepath.Join(root, "paisa.db"),
		"financial_year_starting_month": 4,
	})
	require.NoError(t, err)
	require.NoError(t, config.LoadConfig(content, ""))
}

func investmentYearStart(offset int) time.Time {
	return utils.BeginningOfFinancialYear(utils.Now()).AddDate(offset, 0, 0)
}

func investmentPosting(offsetYears int, monthOffset int, account string, amount int64) posting.Posting {
	return posting.Posting{
		Date:      investmentYearStart(offsetYears).AddDate(0, monthOffset, 0),
		Account:   account,
		Commodity: "INR",
		Amount:    decimal.NewFromInt(amount),
		Quantity:  decimal.NewFromInt(amount),
	}
}

func TestComputeInvestmentYearlyCardSubtractsTaxesOnce(t *testing.T) {
	configureInvestmentTest(t)

	cards := computeInvestmentYearlyCard(
		investmentYearStart(0),
		nil,
		[]posting.Posting{
			investmentPosting(0, 2, "Expenses:Food", 600),
			investmentPosting(0, 3, "Expenses:Tax", 100),
		},
		[]posting.Posting{
			investmentPosting(0, 1, "Income:Salary", -1000),
		},
	)

	require.Len(t, cards, 1)
	card := cards[0]
	require.True(t, card.NetIncome.Equal(decimal.NewFromInt(900)), "net income should only subtract tax once, got %s", card.NetIncome)
	require.True(t, card.NetExpense.Equal(decimal.NewFromInt(600)), "tax must not appear in net expense, got %s", card.NetExpense)
	require.True(t, card.OverallSurplus.Equal(decimal.NewFromInt(300)), "surplus should be net income minus net expense, got %s", card.OverallSurplus)
	require.True(t, card.OverallSavingsRateAvailable, "overall savings rate should be available for positive net income")
	require.True(t, card.OverallSavingsRate.Sub(decimal.RequireFromString("33.3333")).Abs().LessThan(decimal.RequireFromString("0.001")),
		"overall savings rate should be 100 * surplus / net income, got %s", card.OverallSavingsRate)
}

func TestComputeInvestmentYearlyCardKeepsInvestmentRateSeparate(t *testing.T) {
	configureInvestmentTest(t)

	cards := computeInvestmentYearlyCard(
		investmentYearStart(0),
		[]posting.Posting{investmentPosting(0, 4, "Assets:Stocks", 200)},
		[]posting.Posting{investmentPosting(0, 3, "Expenses:Tax", 100)},
		[]posting.Posting{investmentPosting(0, 1, "Income:Salary", -1000)},
	)

	require.Len(t, cards, 1)
	card := cards[0]
	require.True(t, card.NetInvestment.Equal(decimal.NewFromInt(200)), "net investment should keep its own value, got %s", card.NetInvestment)
	require.True(t, card.SavingsRate.Sub(decimal.RequireFromString("22.2222")).Abs().LessThan(decimal.RequireFromString("0.001")),
		"savings rate should stay 100 * net investment / net income, got %s", card.SavingsRate)
	require.True(t, card.OverallSavingsRate.Sub(decimal.NewFromInt(100)).Abs().LessThan(decimal.RequireFromString("0.001")),
		"overall savings rate should describe the full surplus, got %s", card.OverallSavingsRate)
}

func TestComputeInvestmentYearlyCardMarksCurrentFinancialYearIncomplete(t *testing.T) {
	configureInvestmentTest(t)

	cards := computeInvestmentYearlyCard(
		investmentYearStart(-1),
		[]posting.Posting{investmentPosting(-1, 1, "Assets:Stocks", 200)},
		nil,
		[]posting.Posting{
			investmentPosting(-1, 1, "Income:Salary", -1000),
			investmentPosting(0, 1, "Income:Salary", -1000),
		},
	)

	require.Len(t, cards, 2)
	require.True(t, cards[0].PeriodComplete, "a past financial year should be complete")
	require.False(t, cards[1].PeriodComplete, "the running financial year should not be marked complete")
}

func TestComputeInvestmentYearlyCardWorksWithoutInvestmentPostings(t *testing.T) {
	configureInvestmentTest(t)

	cards := computeInvestmentYearlyCard(
		investmentYearStart(0),
		nil,
		[]posting.Posting{investmentPosting(0, 2, "Expenses:Food", 600)},
		[]posting.Posting{investmentPosting(0, 1, "Income:Salary", -1000)},
	)

	require.Len(t, cards, 1, "income and expense postings alone must produce a yearly card")
	card := cards[0]
	require.True(t, card.NetInvestment.Equal(decimal.Zero), "net investment should be zero without investment postings")
	require.True(t, card.SavingsRate.Equal(decimal.Zero), "investment rate should be zero percent for positive net income")
	require.True(t, card.HasActivity, "income and expense postings count as activity")
}

func TestComputeInvestmentYearlyCardMarksEmptyPeriodAsNoActivity(t *testing.T) {
	configureInvestmentTest(t)

	cards := computeInvestmentYearlyCard(
		investmentYearStart(-2),
		nil,
		[]posting.Posting{investmentPosting(0, 2, "Expenses:Food", 1200)},
		[]posting.Posting{
			investmentPosting(-2, 1, "Income:Salary", -1000),
			investmentPosting(0, 1, "Income:Salary", -1000),
		},
	)

	require.Len(t, cards, 3)
	require.True(t, cards[0].HasActivity, "the first period has recorded postings")
	require.False(t, cards[1].HasActivity, "a period without relevant postings must not claim activity")
	require.True(t, cards[2].HasActivity, "a recorded deficit still counts as activity")
	require.True(t, cards[2].OverallSurplus.Equal(decimal.NewFromInt(-200)), "recorded deficit should stay visible, got %s", cards[2].OverallSurplus)
	require.True(t, cards[2].OverallSavingsRate.LessThan(decimal.Zero), "recorded deficit should produce a negative rate, got %s", cards[2].OverallSavingsRate)
}

func openInvestmentTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:investment-%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&posting.Posting{}))
	return db
}

func TestGetInvestmentReturnsNothingForEmptyJournal(t *testing.T) {
	configureInvestmentTest(t)

	response := GetInvestment(openInvestmentTestDB(t))

	cards, ok := response["yearly_cards"].([]InvestmentYearlyCard)
	require.True(t, ok)
	require.Empty(t, cards, "a completely empty journal should not produce yearly cards")
}

func TestGetInvestmentCreatesNoActivityCardForUnrelatedPostings(t *testing.T) {
	configureInvestmentTest(t)
	db := openInvestmentTestDB(t)
	require.NoError(t, db.Create(&posting.Posting{
		Date:      investmentYearStart(0).AddDate(0, 1, 0),
		Account:   "Assets:Checking",
		Commodity: "INR",
		Amount:    decimal.NewFromInt(500),
		Quantity:  decimal.NewFromInt(500),
	}).Error)

	response := GetInvestment(db)

	cards, ok := response["yearly_cards"].([]InvestmentYearlyCard)
	require.True(t, ok)
	require.Len(t, cards, 1, "a non-empty journal should produce a card even without relevant postings")
	require.False(t, cards[0].HasActivity, "unrelated postings do not count as activity")
}

func TestComputeInvestmentYearlyCardMarksRecordedIncomeAndExpenseSides(t *testing.T) {
	configureInvestmentTest(t)

	cases := []struct {
		name          string
		expenses      []posting.Posting
		incomes       []posting.Posting
		hasIncome     bool
		hasExpense    bool
		unavailableOn string
	}{
		{
			name:          "income only",
			incomes:       []posting.Posting{investmentPosting(0, 1, "Income:Salary", -1000)},
			hasIncome:     true,
			hasExpense:    false,
			unavailableOn: "expense side",
		},
		{
			name:          "expense only",
			expenses:      []posting.Posting{investmentPosting(0, 2, "Expenses:Food", 600)},
			hasIncome:     false,
			hasExpense:    true,
			unavailableOn: "income side",
		},
		{
			name:       "both sides",
			expenses:   []posting.Posting{investmentPosting(0, 2, "Expenses:Food", 600)},
			incomes:    []posting.Posting{investmentPosting(0, 1, "Income:Salary", -1000)},
			hasIncome:  true,
			hasExpense: true,
		},
		{
			name:          "tax only is not an expense side",
			expenses:      []posting.Posting{investmentPosting(0, 2, "Expenses:Tax", 100)},
			incomes:       []posting.Posting{investmentPosting(0, 1, "Income:Salary", -1000)},
			hasIncome:     true,
			hasExpense:    false,
			unavailableOn: "expense side",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cards := computeInvestmentYearlyCard(investmentYearStart(0), nil, tc.expenses, tc.incomes)

			require.Len(t, cards, 1)
			require.Equal(t, tc.hasIncome, cards[0].HasIncomePostings, "unexpected income availability for %s", tc.unavailableOn)
			require.Equal(t, tc.hasExpense, cards[0].HasExpensePostings, "unexpected expense availability for %s", tc.unavailableOn)
		})
	}
}
