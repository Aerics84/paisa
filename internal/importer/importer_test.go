package importer

import (
	"strings"
	"testing"
	"time"

	"github.com/ananthakumaran/paisa/internal/accounting"
	"github.com/ananthakumaran/paisa/internal/model/import_rule"
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	accounting.ClearCache()
	t.Cleanup(accounting.ClearCache)

	name := strings.NewReplacer("/", "_", "\\", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&posting.Posting{}, &import_rule.Rule{}))
	return db
}

func TestDetectHeaderRow(t *testing.T) {
	records := [][]string{
		{"", "", "", ""},
		{"Transaction Date", "Details", "Amount (INR)", "Reference Number"},
		{"11-10-2026", "UPI-Smart Ba", "366.55 Dr.", "14336279452"},
	}

	headerIdx, dateIdx, descIdx, debitIdx, creditIdx, amountIdx, currIdx, currency := detectHeaderRow(records)
	assert.Equal(t, 1, headerIdx)
	assert.Equal(t, 0, dateIdx)
	assert.Equal(t, 1, descIdx)
	assert.Equal(t, -1, debitIdx)
	assert.Equal(t, -1, creditIdx)
	assert.Equal(t, 2, amountIdx)
	assert.Equal(t, -1, currIdx)
	assert.Equal(t, "INR", currency)

	debitCreditRecords := [][]string{
		{"Date", "Description", "Debit", "Credit"},
		{"2026-09-15", "Netflix Subscription", "19.99", ""},
	}
	headerIdx, dateIdx, descIdx, debitIdx, creditIdx, amountIdx, _, _ = detectHeaderRow(debitCreditRecords)
	assert.Equal(t, 0, headerIdx)
	assert.Equal(t, 0, dateIdx)
	assert.Equal(t, 1, descIdx)
	assert.Equal(t, 2, debitIdx)
	assert.Equal(t, 3, creditIdx)
	assert.Equal(t, -1, amountIdx)

	headerIdx, dateIdx, _, _, _, _, _, _ = detectHeaderRow([][]string{
		{"Some", "Random", "Text"},
		{"without", "a", "table"},
	})
	assert.Equal(t, -1, headerIdx)
	assert.Equal(t, -1, dateIdx)
}

func TestParseCSVDebitCreditColumns(t *testing.T) {
	db := openTestDB(t)

	content := `Date,Description,Debit,Credit
2026-09-15,Netflix Subscription,19.99,
2026-09-16,Salary Deposit,,3500.00
`

	candidates, err := parseCSVRows(content, "", "", db)
	require.NoError(t, err)
	require.Len(t, candidates, 2)

	assert.Equal(t, "2026-09-15", candidates[0].Date)
	assert.Equal(t, "Netflix Subscription", candidates[0].Payee)
	assert.Equal(t, 19.99, candidates[0].Amount)
	assert.True(t, candidates[0].IsDebit)

	assert.Equal(t, "2026-09-16", candidates[1].Date)
	assert.Equal(t, "Salary Deposit", candidates[1].Payee)
	assert.Equal(t, 3500.00, candidates[1].Amount)
	assert.False(t, candidates[1].IsDebit)
}

func TestParseCSVAmountWithDrCr(t *testing.T) {
	db := openTestDB(t)

	statement := `,,,,,,,,,,,,,,,,
,VIEW CURRENT STATEMENT,,,,,,,,,,,,,,,,
,,,,,,,,,,,,,,,,
,My Credit Card Details for,,,,XXXAFAFA 450172******0023,,,,,,,,,,,,
,Card Holder Name,,,,XXXAFAFA,,,,Card Type,,,,primary,,,,
,Statement Date,,,,12-10-2026,,,,Previous Balance,,,,INR 4083,,,,
,Transaction Details,,,,,,,,,,,,,,,,
,Transaction Date,,,,Details,,,,Amount (INR),,,,Reward Points,,,,Reference Number
,,,,,,,,,,,,,,,,
,11-10-2026,,,,UPI-Smart Ba,,,,366.55 Dr.,,,,7,,,,14336279452
,10-10-2026,,,,AMAZON MUMBAI,,,,299 Dr.,,,,5,,,,14335604658
,26-09-2026,,,,BBPS Payment received,,,,4083 Cr.,,,,0,,,,14238609506
`

	result, err := Ingest(db, IngestionRequest{
		Format:  FormatCSV,
		Content: statement,
	})
	require.NoError(t, err)
	require.Equal(t, 3, result.TotalParsed)

	assert.Equal(t, "2026-10-11", result.Transactions[0].Date)
	assert.Equal(t, "UPI-Smart Ba", result.Transactions[0].Payee)
	assert.Equal(t, 366.55, result.Transactions[0].Amount)
	assert.True(t, result.Transactions[0].IsDebit)

	assert.Equal(t, "2026-10-10", result.Transactions[1].Date)
	assert.Equal(t, 299.0, result.Transactions[1].Amount)
	assert.True(t, result.Transactions[1].IsDebit)

	assert.Equal(t, "2026-09-26", result.Transactions[2].Date)
	assert.Equal(t, "BBPS Payment received", result.Transactions[2].Payee)
	assert.Equal(t, 4083.0, result.Transactions[2].Amount)
	assert.False(t, result.Transactions[2].IsDebit)
}

func TestNormalizeDateString(t *testing.T) {
	cases := []struct {
		input    string
		expected string
		ok       bool
	}{
		{"2026-10-11", "2026-10-11", true},
		{"11-10-2026", "2026-10-11", true},
		{"11/10/2026", "2026-10-11", true},
		{"11-Oct-2026", "2026-10-11", true},
		{"11 Oct 2026", "2026-10-11", true},
		{"2026/10/11", "2026-10-11", true},
		{"2026-10-11 15:04:05", "2026-10-11", true},
		{"11/10/2026 15:04:05", "2026-10-11", true},
		{"11/10/2026 15:04", "2026-10-11", true},
		{" 2026-10-11 ", "2026-10-11", true},
		{"Total", "", false},
		{"Closing Balance", "", false},
		{"", "", false},
	}

	for _, tc := range cases {
		actual, ok := normalizeDateString(tc.input)
		assert.Equal(t, tc.ok, ok, "unexpected parse result for %q", tc.input)
		assert.Equal(t, tc.expected, actual, "unexpected date for %q", tc.input)
	}
}

func TestParseCSVSkipsSummaryRows(t *testing.T) {
	db := openTestDB(t)

	content := `Date,Description,Debit,Credit
2026-09-15,Coffee Shop,150.00,
Total,,150.00,
Closing Balance,,5000.00,
`

	result, err := Ingest(db, IngestionRequest{
		Format:  FormatCSV,
		Content: content,
	})
	require.NoError(t, err)

	assert.Equal(t, 1, result.TotalParsed)
	require.Len(t, result.Transactions, 1)
	assert.Equal(t, "Coffee Shop", result.Transactions[0].Payee)
	assert.Equal(t, 150.00, result.Transactions[0].Amount)
}

func TestIngestOFX(t *testing.T) {
	db := openTestDB(t)

	content := `<OFX>
<BANKMSGSRSV1>
<STMTTRNRS>
<STMTRS>
<CURDEF>USD
<BANKACCTFROM>
<ACCTID>123456789
</BANKACCTFROM>
<BANKTRANLIST>
<STMTTRN>
<TRNTYPE>DEBIT
<DTPOSTED>20261011
<TRNAMT>-45.20
<FITID>2026101101
<NAME>Coffee Shop
<MEMO>Card purchase
</STMTTRN>
<STMTTRN>
<TRNTYPE>CREDIT
<DTPOSTED>20261012120000
<TRNAMT>3500.00
<FITID>2026101201
<NAME>Salary Deposit
</STMTTRN>
</BANKTRANLIST>
</STMTRS>
</STMTTRNRS>
</BANKMSGSRSV1>
</OFX>`

	result, err := Ingest(db, IngestionRequest{
		Format:  FormatOFX,
		Content: content,
	})
	require.NoError(t, err)
	require.Equal(t, 2, result.TotalParsed)

	assert.Equal(t, "2026-10-11", result.Transactions[0].Date)
	assert.Equal(t, "Coffee Shop", result.Transactions[0].Payee)
	assert.Equal(t, "Card purchase", result.Transactions[0].Memo)
	assert.Equal(t, 45.20, result.Transactions[0].Amount)
	assert.True(t, result.Transactions[0].IsDebit)
	assert.Equal(t, "USD", result.Transactions[0].Currency)

	assert.Equal(t, "2026-10-12", result.Transactions[1].Date)
	assert.Equal(t, 3500.00, result.Transactions[1].Amount)
	assert.False(t, result.Transactions[1].IsDebit)
	assert.Equal(t, "USD", result.Transactions[1].Currency)
}

func TestLoadPostingsForIngestionDateWindow(t *testing.T) {
	db := openTestDB(t)

	makePosting := func(date string, payee string) {
		txDate, err := time.Parse("2006-01-02", date)
		require.NoError(t, err)
		require.NoError(t, db.Create(&posting.Posting{
			Date:      txDate,
			Payee:     payee,
			Amount:    decimal.NewFromFloat(-45.20),
			Commodity: "INR",
			Account:   "Assets:Checking",
		}).Error)
	}

	makePosting("2026-08-01", "Too Old")
	makePosting("2026-10-10", "Coffee Shop")
	makePosting("2026-10-19", "Too New")

	postings := loadPostingsForIngestion(db, []import_rule.CandidateTransaction{
		{Date: "2026-10-11", Payee: "Coffee Shop", Amount: 45.20, IsDebit: true},
	})

	require.Len(t, postings, 1)
	assert.Equal(t, "Coffee Shop", postings[0].Payee)
}

func TestIngestDuplicateDateWindow(t *testing.T) {
	db := openTestDB(t)

	inWindowDate, err := time.Parse("2006-01-02", "2026-10-10")
	require.NoError(t, err)
	inWindow := &posting.Posting{
		Date:      inWindowDate,
		Payee:     "Netflix Subscription",
		Amount:    decimal.NewFromFloat(-19.99),
		Commodity: "INR",
		Account:   "Assets:Checking",
	}
	require.NoError(t, db.Create(inWindow).Error)

	outOfWindowDate, err := time.Parse("2006-01-02", "2026-08-01")
	require.NoError(t, err)
	require.NoError(t, db.Create(&posting.Posting{
		Date:      outOfWindowDate,
		Payee:     "Netflix Subscription",
		Amount:    decimal.NewFromFloat(-19.99),
		Commodity: "INR",
		Account:   "Assets:Checking",
	}).Error)

	result, err := Ingest(db, IngestionRequest{
		Format:  FormatCSV,
		Content: "Date,Description,Debit,Credit\n2026-10-11,Netflix Subscription,19.99,\n",
	})
	require.NoError(t, err)
	require.Len(t, result.Transactions, 1)

	tx := result.Transactions[0]
	assert.True(t, tx.IsDuplicate)
	assert.GreaterOrEqual(t, tx.DuplicateScore, 70)
	assert.Equal(t, inWindow.ID, tx.DuplicateMatchID)
	assert.Equal(t, 1, result.PossibleDuplicates)
}

func duplicateTestPosting(t *testing.T, date, payee string, amount float64) posting.Posting {
	t.Helper()

	txDate, err := time.Parse("2006-01-02", date)
	require.NoError(t, err)

	return posting.Posting{
		ID:        42,
		Date:      txDate,
		Payee:     payee,
		Amount:    decimal.NewFromFloat(-amount),
		Commodity: "INR",
		Account:   "Assets:Checking",
	}
}

func TestDuplicateScoreExactMatch(t *testing.T) {
	existing := []posting.Posting{duplicateTestPosting(t, "2026-10-11", "SWIGGY", 500)}

	match, score, reason := findBestDuplicateMatch(import_rule.CandidateTransaction{
		Date:   "2026-10-11",
		Payee:  "SWIGGY",
		Amount: 500,
	}, existing)

	require.NotNil(t, match)
	assert.Equal(t, uint(42), match.ID)
	assert.GreaterOrEqual(t, score, 70)
	assert.Contains(t, reason, "exact amount match")
	assert.Contains(t, reason, "same day")
	assert.Contains(t, reason, "payee text match")
}

func TestDuplicateScoreRejectsOldDate(t *testing.T) {
	existing := []posting.Posting{duplicateTestPosting(t, "2026-10-01", "SWIGGY", 500)}

	_, score, _ := findBestDuplicateMatch(import_rule.CandidateTransaction{
		Date:   "2026-10-11",
		Payee:  "SWIGGY",
		Amount: 500,
	}, existing)

	assert.Less(t, score, 70)
}

func TestDuplicateScoreRejectsDifferentMerchant(t *testing.T) {
	existing := []posting.Posting{duplicateTestPosting(t, "2026-10-11", "AMAZON", 500)}

	_, score, _ := findBestDuplicateMatch(import_rule.CandidateTransaction{
		Date:   "2026-10-11",
		Payee:  "SWIGGY",
		Amount: 500,
	}, existing)

	assert.Less(t, score, 70)
}

func TestIngestCSVWithRulesAndDuplicates(t *testing.T) {
	db := openTestDB(t)

	existingTxDate, err := time.Parse("2006-01-02", "2026-09-15")
	require.NoError(t, err)
	require.NoError(t, db.Create(&posting.Posting{
		Date:      existingTxDate,
		Payee:     "Netflix Subscription",
		Amount:    decimal.NewFromFloat(-19.99),
		Commodity: "USD",
		Account:   "Assets:Checking",
	}).Error)

	require.NoError(t, import_rule.Save(db, &import_rule.Rule{
		Name:          "Starbucks Rule",
		PayeePattern:  "Starbucks",
		TargetAccount: "Expenses:Dining:Coffee",
		Tags:          []string{"#coffee"},
		Priority:      10,
	}))

	csvData := `Date,Description,Debit,Credit
2026-09-15,Netflix Subscription,19.99,
2026-09-16,Starbucks Store #123,5.75,
2026-09-20,Unknown Merchant,50.00,
`

	result, err := Ingest(db, IngestionRequest{
		Format:  FormatCSV,
		Content: csvData,
	})
	require.NoError(t, err)
	require.Equal(t, 3, result.TotalParsed)

	tx1 := result.Transactions[0]
	assert.Equal(t, "Netflix Subscription", tx1.Payee)
	assert.True(t, tx1.IsDuplicate)
	assert.GreaterOrEqual(t, tx1.DuplicateScore, 70)

	tx2 := result.Transactions[1]
	assert.Equal(t, "Starbucks Store #123", tx2.Payee)
	assert.Equal(t, "Expenses:Dining:Coffee", tx2.SelectedAccount)
	assert.Equal(t, "Starbucks Rule", tx2.MatchedRuleName)
	assert.Equal(t, []string{"#coffee"}, tx2.Tags)
	assert.False(t, tx2.IsDuplicate)

	tx3 := result.Transactions[2]
	assert.Equal(t, "", tx3.SelectedAccount)
	assert.False(t, tx3.IsDuplicate)
}
