package assets

import (
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

func balanceTestDate(date string) time.Time {
	parsed, err := time.ParseInLocation("2006-01-02", date, config.TimeZone())
	if err != nil {
		panic(err)
	}
	return parsed
}

func openBalanceTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	err := config.LoadConfig([]byte(`
journal_path: main.ledger
db_path: paisa.db
default_currency: EUR
time_zone: UTC
commodities:
  - name: AIB Group
    type: stock
    price:
      provider: com-yahoo
      code: AIBRF
  - name: OTHER
    type: stock
    price:
      provider: com-yahoo
      code: OTHER
  - name: DISPLAY
    display_name: Display Name
    type: stock
    price:
      provider: com-yahoo
      code: DISP
`), "")
	require.NoError(t, err)

	name := t.Name()
	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&posting.Posting{}))
	return db
}

func addCheckingPosting(t *testing.T, db *gorm.DB, transactionID string, date string, account string, amount string) {
	t.Helper()

	require.NoError(t, db.Create(&posting.Posting{
		TransactionID: transactionID,
		Date:          balanceTestDate(date),
		Account:       account,
		Commodity:     "EUR",
		Quantity:      decimal.RequireFromString(amount),
		Amount:        decimal.RequireFromString(amount),
	}).Error)
}

func findCheckingBalance(points []CheckingBalancePoint, month string, account string) (decimal.Decimal, bool) {
	for _, point := range points {
		if point.Month.Format("2006-01") == month && point.Account == account {
			return point.Balance, true
		}
	}
	return decimal.Zero, false
}

func TestCheckingBalanceTimelineTracksMultipleAccounts(t *testing.T) {
	db := openBalanceTestDB(t)

	addCheckingPosting(t, db, "salary", "2022-01-10", "Assets:Checking:BankA", "1000")
	addCheckingPosting(t, db, "savings", "2022-01-15", "Assets:Checking:BankB", "500")
	addCheckingPosting(t, db, "transfer", "2022-02-05", "Assets:Checking:BankA", "-100")
	addCheckingPosting(t, db, "transfer", "2022-02-05", "Assets:Checking:BankB", "100")
	addCheckingPosting(t, db, "expense", "2022-02-20", "Assets:Checking:BankA", "-200")

	points := GetCheckingBalanceTimeline(db)

	balance, found := findCheckingBalance(points, "2022-01", "Assets:Checking:BankA")
	require.True(t, found)
	require.True(t, balance.Equal(decimal.NewFromInt(1000)))
	balance, found = findCheckingBalance(points, "2022-01", "Assets:Checking:BankB")
	require.True(t, found)
	require.True(t, balance.Equal(decimal.NewFromInt(500)))

	balance, found = findCheckingBalance(points, "2022-02", "Assets:Checking:BankA")
	require.True(t, found)
	require.True(t, balance.Equal(decimal.NewFromInt(700)))
	balance, found = findCheckingBalance(points, "2022-02", "Assets:Checking:BankB")
	require.True(t, found)
	require.True(t, balance.Equal(decimal.NewFromInt(600)))
}

func TestCheckingBalanceTimelineKeepsInternalTransfersNeutral(t *testing.T) {
	db := openBalanceTestDB(t)

	addCheckingPosting(t, db, "open-a", "2022-01-01", "Assets:Checking:BankA", "1000")
	addCheckingPosting(t, db, "open-b", "2022-01-01", "Assets:Checking:BankB", "250")
	addCheckingPosting(t, db, "transfer", "2022-02-01", "Assets:Checking:BankA", "-400")
	addCheckingPosting(t, db, "transfer", "2022-02-01", "Assets:Checking:BankB", "400")

	points := GetCheckingBalanceTimeline(db)

	for _, month := range []string{"2022-01", "2022-02"} {
		total := decimal.Zero
		for _, point := range points {
			if point.Month.Format("2006-01") == month {
				total = total.Add(point.Balance)
			}
		}
		require.True(t, total.Equal(decimal.NewFromInt(1250)), "month %s total should stay 1250, got %s", month, total)
	}
}

func TestCheckingBalanceTimelineLimitsToLatestTwelveRecordedMonths(t *testing.T) {
	db := openBalanceTestDB(t)

	for index := 0; index < 14; index++ {
		month := time.Date(2021, time.January, 10, 0, 0, 0, 0, time.UTC).AddDate(0, index, 0)
		date := month.Format("2006-01-02")
		addCheckingPosting(t, db, "salary-month-"+month.Format("2006-01"), date, "Assets:Checking:Bank", "100")
	}

	points := GetCheckingBalanceTimeline(db)

	months := make(map[string]bool)
	for _, point := range points {
		months[point.Month.Format("2006-01")] = true
	}
	require.Len(t, months, 12)
	require.False(t, months["2021-01"])
	require.False(t, months["2021-02"])
	require.True(t, months["2021-03"])
	require.True(t, months["2022-02"])

	balance, found := findCheckingBalance(points, "2021-03", "Assets:Checking:Bank")
	require.True(t, found)
	require.True(t, balance.Equal(decimal.NewFromInt(300)), "the carried balance must include all earlier postings")
}

func TestCheckingBalanceTimelineFillsEmptyCalendarMonths(t *testing.T) {
	db := openBalanceTestDB(t)

	addCheckingPosting(t, db, "jan", "2022-01-10", "Assets:Checking:Bank", "1000")
	addCheckingPosting(t, db, "mar", "2022-03-10", "Assets:Checking:Bank", "500")

	points := GetCheckingBalanceTimeline(db)

	balance, found := findCheckingBalance(points, "2022-02", "Assets:Checking:Bank")
	require.True(t, found, "an empty calendar month must still carry the recorded balance")
	require.True(t, balance.Equal(decimal.NewFromInt(1000)))
	balance, found = findCheckingBalance(points, "2022-03", "Assets:Checking:Bank")
	require.True(t, found)
	require.True(t, balance.Equal(decimal.NewFromInt(1500)))
}

func TestCheckingBalanceTimelineReturnsEmptyWithoutCheckingAccounts(t *testing.T) {
	db := openBalanceTestDB(t)

	addCheckingPosting(t, db, "investment", "2022-01-10", "Assets:Equity:VWCE", "1000")

	points := GetCheckingBalanceTimeline(db)
	require.Empty(t, points)
}

func TestAvailableMonthRangeUsesRecordedPostingBounds(t *testing.T) {
	db := openBalanceTestDB(t)
	utils.SetNow("2022-03-15")

	addCheckingPosting(t, db, "jan", "2020-02-10", "Assets:Checking:Bank", "100")
	addCheckingPosting(t, db, "feb", "2022-04-20", "Assets:Checking:Bank", "100")

	monthRange := GetAvailableMonthRange(db)

	require.True(t, monthRange.From.Equal(utils.BeginningOfMonth(balanceTestDate("2020-02-10"))))
	require.True(t, monthRange.To.Equal(utils.BeginningOfMonth(balanceTestDate("2022-04-20"))))
}

func TestAvailableMonthRangeIncludesCurrentMonthWithoutPostings(t *testing.T) {
	db := openBalanceTestDB(t)
	utils.SetNow("2022-03-15")

	addCheckingPosting(t, db, "jan", "2022-01-10", "Assets:Checking:Bank", "100")

	monthRange := GetAvailableMonthRange(db)

	require.True(t, monthRange.To.Equal(utils.BeginningOfMonth(balanceTestDate("2022-03-15"))), "the current month must remain selectable for month-to-date")
}

func TestAvailableMonthRangeIsZeroWithoutPostings(t *testing.T) {
	db := openBalanceTestDB(t)

	monthRange := GetAvailableMonthRange(db)
	require.True(t, monthRange.From.IsZero())
	require.True(t, monthRange.To.IsZero())
}

func TestComputeBreakdownsIncludesConfiguredCommodityMetadata(t *testing.T) {
	db := openBalanceTestDB(t)
	postings := []posting.Posting{
		{
			TransactionID: "buy",
			Date:          balanceTestDate("2025-06-10"),
			Account:       "Assets:Investments:IE00BF0L3536",
			Commodity:     "AIB Group",
			Quantity:      decimal.NewFromInt(10),
			Amount:        decimal.NewFromInt(1000),
		},
	}

	breakdowns := ComputeBreakdowns(db, postings, true)
	leaf := breakdowns["Assets:Investments:IE00BF0L3536"]

	require.Equal(t, "AIB Group", leaf.Commodity)
	require.Equal(t, "AIB Group", leaf.CommodityName)
	require.Equal(t, "AIBRF", leaf.CommodityCode)
}

func TestComputeBreakdownsUsesCommodityDisplayName(t *testing.T) {
	db := openBalanceTestDB(t)
	postings := []posting.Posting{
		{
			TransactionID: "buy",
			Date:          balanceTestDate("2025-06-10"),
			Account:       "Assets:Investments:DISPLAY",
			Commodity:     "DISPLAY",
			Quantity:      decimal.NewFromInt(10),
			Amount:        decimal.NewFromInt(1000),
		},
	}

	breakdowns := ComputeBreakdowns(db, postings, true)
	leaf := breakdowns["Assets:Investments:DISPLAY"]

	require.Equal(t, "DISPLAY", leaf.Commodity)
	require.Equal(t, "Display Name", leaf.CommodityName)
	require.Equal(t, "DISP", leaf.CommodityCode)
}

func TestComputeBreakdownsOmitsUnknownOrMixedCommodities(t *testing.T) {
	db := openBalanceTestDB(t)
	unknown := posting.Posting{
		TransactionID: "buy-unknown", Date: balanceTestDate("2025-06-10"),
		Account: "Assets:Investments:UNKNOWN", Commodity: "UNKNOWN",
		Quantity: decimal.NewFromInt(1), Amount: decimal.NewFromInt(10),
	}
	mixedA := posting.Posting{
		TransactionID: "buy-a", Date: balanceTestDate("2025-06-10"),
		Account: "Assets:Investments:MIXED", Commodity: "AIB Group",
		Quantity: decimal.NewFromInt(1), Amount: decimal.NewFromInt(10),
	}
	mixedB := mixedA
	mixedB.Commodity = "OTHER"

	breakdowns := ComputeBreakdowns(db, []posting.Posting{unknown, mixedA, mixedB}, true)
	require.Empty(t, breakdowns["Assets:Investments:UNKNOWN"].Commodity)
	require.Empty(t, breakdowns["Assets:Investments:MIXED"].Commodity)
}
