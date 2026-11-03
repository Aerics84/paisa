package importer

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ananthakumaran/paisa/internal/accounting"
	"github.com/ananthakumaran/paisa/internal/config"
	importmanifest "github.com/ananthakumaran/paisa/internal/importer/manifest"
	"github.com/ananthakumaran/paisa/internal/importer/reconciliation"
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

	headerIdx, dateIdx, descIdx, debitIdx, creditIdx, amountIdx, drCrIdx, currIdx, fallbackDateIdx, currency := detectHeaderRow(records)
	assert.Equal(t, 1, headerIdx)
	assert.Equal(t, 0, dateIdx)
	assert.Equal(t, 1, descIdx)
	assert.Equal(t, -1, debitIdx)
	assert.Equal(t, -1, creditIdx)
	assert.Equal(t, 2, amountIdx)
	assert.Equal(t, -1, drCrIdx)
	assert.Equal(t, -1, currIdx)
	assert.Equal(t, -1, fallbackDateIdx)
	assert.Equal(t, "INR", currency)

	debitCreditRecords := [][]string{
		{"Date", "Description", "Debit", "Credit"},
		{"2026-09-15", "Netflix Subscription", "19.99", ""},
	}
	headerIdx, dateIdx, descIdx, debitIdx, creditIdx, amountIdx, drCrIdx, _, fallbackDateIdx, _ = detectHeaderRow(debitCreditRecords)
	assert.Equal(t, 0, headerIdx)
	assert.Equal(t, 0, dateIdx)
	assert.Equal(t, 1, descIdx)
	assert.Equal(t, 2, debitIdx)
	assert.Equal(t, 3, creditIdx)
	assert.Equal(t, -1, amountIdx)
	assert.Equal(t, -1, drCrIdx)
	assert.Equal(t, -1, fallbackDateIdx)

	kotakRecords := [][]string{
		{"Sl. No,", "Transaction Date", "Value Date", "Description", "Chq /Ref No.", "Amount", "Dr / Cr", "Balance", "Dr / Cr"},
		{"1", "25-10-26 23:46", "25-10-26", "MEESHO TECHNOLOGIES", "6.29823E+11", "172", "DR", "808.4", "CR"},
	}
	headerIdx, dateIdx, _, _, _, amountIdx, drCrIdx, _, fallbackDateIdx, _ = detectHeaderRow(kotakRecords)
	assert.Equal(t, 0, headerIdx)
	assert.Equal(t, 1, dateIdx)
	assert.Equal(t, 5, amountIdx)
	assert.Equal(t, 6, drCrIdx)
	assert.Equal(t, 2, fallbackDateIdx)

	headerIdx, dateIdx, _, _, _, _, _, _, _, _ = detectHeaderRow([][]string{
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

	candidates, err := parseCSVRows(content, "", "", db, "en-IN", nil)
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

// The Kotak fixture is rendered against
// internal/model/template/templates/Kotak Account Statement.handlebars by the
// TypeScript fixture harness in src/lib/import.test.ts. This test pins the Go
// side of that pipeline: the CSV parser must recover the dates, amounts and
// Dr/Cr direction that the template turns into monthly.ledger.
func TestIngestKotakStatement(t *testing.T) {
	db := openTestDB(t)

	content, err := os.ReadFile("../../fixture/import/Kotak Account Statement/monthly.csv")
	require.NoError(t, err)

	result, err := Ingest(db, IngestionRequest{
		Format:  FormatCSV,
		Content: string(content),
	})
	require.NoError(t, err)
	require.Equal(t, 2, result.TotalParsed)

	tx0 := result.Transactions[0]
	assert.Equal(t, "2026-10-25", tx0.Date)
	assert.Equal(t, "MEESHO TECHNOLOGIES", tx0.Payee)
	assert.Equal(t, 172.0, tx0.Amount)
	assert.True(t, tx0.IsDebit, "DR indicator must mark the amount as debit")
	assert.Equal(t, "INR", tx0.Currency)

	tx1 := result.Transactions[1]
	assert.Equal(t, "2026-10-25", tx1.Date)
	assert.Equal(t, "CASHBACK EARNED", tx1.Payee)
	assert.Equal(t, 1.5, tx1.Amount)
	assert.False(t, tx1.IsDebit, "CR indicator must mark the amount as credit")
	assert.Equal(t, "INR", tx1.Currency)
}

func TestKotakValueDateFallback(t *testing.T) {
	db := openTestDB(t)

	content := `Transaction Date,Value Date,Description,Amount,Dr / Cr
25-10-26T23:46,25-10-26,MEESHO TECHNOLOGIES,172,DR
`

	result, err := Ingest(db, IngestionRequest{
		Format:  FormatCSV,
		Content: content,
	})
	require.NoError(t, err)
	require.Equal(t, 1, result.TotalParsed)
	assert.Equal(t, "2026-10-25", result.Transactions[0].Date)
	assert.True(t, result.Transactions[0].IsDebit)
}

// The ICICI bank statement fixture mirrors the normalized CSV produced from
// the bank's .xls export (DD/MM/YYYY dates and separate debit/credit columns).
// It is rendered against internal/model/template/templates/ICICI Bank
// Statement.handlebars by the TypeScript fixture harness; this test pins the Go
// side of the pipeline so the parser keeps recovering dates, amounts, thousands
// separators and the debit/credit direction.
func TestIngestICICIBankStatement(t *testing.T) {
	db := openTestDB(t)

	content, err := os.ReadFile("../../fixture/import/ICICI Bank Statement/monthly.csv")
	require.NoError(t, err)

	result, err := Ingest(db, IngestionRequest{
		Format:  FormatCSV,
		Content: string(content),
	})
	require.NoError(t, err)
	require.Equal(t, 5, result.TotalParsed)

	tx0 := result.Transactions[0]
	assert.Equal(t, "2026-10-01", tx0.Date)
	assert.Equal(t, "UPI/DR/415923876543/SWIGGY/YESB/swiggy@ybl", tx0.Payee)
	assert.Equal(t, 450.0, tx0.Amount)
	assert.True(t, tx0.IsDebit, "empty credit column must keep the row a debit")
	assert.Equal(t, "INR", tx0.Currency)

	tx1 := result.Transactions[1]
	assert.Equal(t, "2026-10-03", tx1.Date)
	assert.Equal(t, "NEFT/ICIC0000123/ACME PAYROLL/SALARY OCT 2026", tx1.Payee)
	assert.Equal(t, 85000.0, tx1.Amount)
	assert.False(t, tx1.IsDebit, "empty debit column must mark the row as credit")
	assert.Equal(t, "INR", tx1.Currency)

	tx2 := result.Transactions[2]
	assert.Equal(t, "2026-10-07", tx2.Date)
	assert.Equal(t, 1500.0, tx2.Amount)
	assert.False(t, tx2.IsDebit)

	tx3 := result.Transactions[3]
	assert.Equal(t, "2026-10-12", tx3.Date)
	assert.Equal(t, 2499.0, tx3.Amount)
	assert.True(t, tx3.IsDebit)

	tx4 := result.Transactions[4]
	assert.Equal(t, "2026-10-15", tx4.Date)
	assert.Equal(t, 1234.56, tx4.Amount)
	assert.True(t, tx4.IsDebit)
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
	assert.Equal(t, "Expenses:Unknown", tx3.SelectedAccount)
	assert.Equal(t, "Expenses:Unknown", tx3.SuggestedAccount)
	assert.Equal(t, "fallback", tx3.Confidence)
	assert.False(t, tx3.IsDuplicate)
}
func useTempIngestConfigDir(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	previousConfigDir := ingestConfigDir
	ingestConfigDir = func() string { return root }
	t.Cleanup(func() { ingestConfigDir = previousConfigDir })
	return root
}

func TestIngestAlreadyImported(t *testing.T) {
	db := openTestDB(t)
	root := useTempIngestConfigDir(t)

	content := `Date,Description,Debit,Credit
2026-09-15,Coffee Shop,150.00,
`
	req := IngestionRequest{
		Format:        FormatCSV,
		Content:       content,
		SourceType:    "bank",
		OriginalPath:  "statements/checking.csv",
		ContentSHA256: sha256HexString(content),
	}

	first, err := Ingest(db, req)
	require.NoError(t, err)
	assert.False(t, first.AlreadyImported)
	assert.Equal(t, 1, first.TotalParsed)
	require.Len(t, first.Transactions, 1)
	require.NotEmpty(t, first.ImportID)

	// Parsing alone only stages the import. Until the manifest is marked
	// committed the same file must stay re-parseable so the UI can retry.
	stagedAgain, err := Ingest(db, req)
	require.NoError(t, err)
	assert.False(t, stagedAgain.AlreadyImported)
	require.Len(t, stagedAgain.Transactions, 1)

	_, err = importmanifest.MarkCommitted(root, req.SourceType, first.ImportID)
	require.NoError(t, err)

	second, err := Ingest(db, req)
	require.NoError(t, err)
	assert.True(t, second.AlreadyImported)
	assert.Equal(t, first.ImportID, second.ImportID)
	assert.Equal(t, 0, second.TotalParsed)
	assert.Empty(t, second.Transactions)

	// The same path with new content is a new import.
	otherContent := `Date,Description,Debit,Credit
2026-09-16,Book Store,42.00,
`
	req.Content = otherContent
	req.ContentSHA256 = sha256HexString(otherContent)
	third, err := Ingest(db, req)
	require.NoError(t, err)
	assert.False(t, third.AlreadyImported)
	require.Len(t, third.Transactions, 1)
}

func TestIngestAlreadyImportedForOlderContent(t *testing.T) {
	db := openTestDB(t)
	root := useTempIngestConfigDir(t)

	contentA := `Date,Description,Debit,Credit
2026-09-15,Coffee Shop,150.00,
`
	contentB := `Date,Description,Debit,Credit
2026-09-16,Book Store,42.00,
`
	reqA := IngestionRequest{
		Format:        FormatCSV,
		Content:       contentA,
		SourceType:    "bank",
		OriginalPath:  "statements/checking.csv",
		ContentSHA256: sha256HexString(contentA),
	}
	reqB := reqA
	reqB.Content = contentB
	reqB.ContentSHA256 = sha256HexString(contentB)

	_, err := Ingest(db, reqA)
	require.NoError(t, err)
	_, err = importmanifest.MarkCommitted(root, reqA.SourceType, importmanifest.ImportID(reqA.SourceType, reqA.OriginalPath, reqA.ContentSHA256))
	require.NoError(t, err)
	_, err = Ingest(db, reqB)
	require.NoError(t, err)

	// A is no longer the latest manifest for the path, but its committed
	// manifest still exists under its deterministic import id.
	again, err := Ingest(db, reqA)
	require.NoError(t, err)
	assert.True(t, again.AlreadyImported)
	assert.Equal(t, 0, again.TotalParsed)
	assert.Empty(t, again.Transactions)
}

func TestIngestReparsesWhenParserVersionChanges(t *testing.T) {
	db := openTestDB(t)
	root := useTempIngestConfigDir(t)

	content := `Date,Description,Debit,Credit
2026-09-17,Grocer,25.00,
`
	req := IngestionRequest{
		Format:        FormatCSV,
		Content:       content,
		SourceType:    "bank",
		OriginalPath:  "statements/parser-version.csv",
		ContentSHA256: sha256HexString(content),
	}

	// Seed the only manifest for this path with an older parser version. The
	// future timestamp guarantees it is the latest entry independent of the
	// wall clock.
	_, err := importmanifest.Write(root, importmanifest.Manifest{
		ImportID:      importmanifest.ImportID(req.SourceType, req.OriginalPath, req.ContentSHA256),
		SourceType:    req.SourceType,
		OriginalPath:  req.OriginalPath,
		ContentSHA256: req.ContentSHA256,
		ImportedAt:    "2099-01-01T00:00:00Z",
		Status:        "staged",
		Parser:        importmanifest.ParserInfo{Name: IngestionParserName, Version: "0"},
	}, []string{})
	require.NoError(t, err)

	result, err := Ingest(db, req)
	require.NoError(t, err)
	assert.False(t, result.AlreadyImported)
	require.Len(t, result.Transactions, 1)

	// Re-ingestion is now idempotent under the current parser version once the
	// staged manifest has been committed.
	_, err = importmanifest.MarkCommitted(root, req.SourceType, result.ImportID)
	require.NoError(t, err)
	again, err := Ingest(db, req)
	require.NoError(t, err)
	assert.True(t, again.AlreadyImported)
	assert.Empty(t, again.Transactions)
}

func sha256HexString(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func TestParseLocaleAmount(t *testing.T) {
	cases := []struct {
		raw      string
		locale   string
		expected float64
	}{
		{"1.234,56", "de-DE", 1234.56},
		{"(1.234,56)", "de-DE", -1234.56},
		{"1.234,56 €", "de-DE", 1234.56},
		{"1 234,56", "fr-FR", 1234.56},
		{"1,23,456.78", "en-IN", 123456.78},
		{"(1,23,456.78)", "en-IN", -123456.78},
		{"₹ 12,34,567.89", "en-IN", 1234567.89},
		{"1.234", "de-DE", 1234},
		{"1234.56", "de-DE", 1234.56},
		{"1,234.56", "en-US", 1234.56},
	}

	for _, tc := range cases {
		actual, ok := parseLocaleAmount(tc.raw, tc.locale)
		require.True(t, ok, "expected %q to parse under %s", tc.raw, tc.locale)
		assert.InDelta(t, tc.expected, actual, 0.001, "unexpected amount for %q under %s", tc.raw, tc.locale)
	}

	_, ok := parseLocaleAmount("not-a-number", "de-DE")
	assert.False(t, ok)
}

func TestParseCSVGermanHeadersAndAmounts(t *testing.T) {
	db := openTestDB(t)

	content := `Datum;Beschreibung;Soll;Haben;Saldo
01.10.2026;Miete;1.234,56;;1.234,56
02.10.2026;Gehalt;;2.500,00;3.734,56
`
	req := IngestionRequest{
		Format:    FormatCSV,
		Content:   content,
		Delimiter: ";",
		Locale:    "de-DE",
	}

	result, err := Ingest(db, req)
	require.NoError(t, err)
	require.Equal(t, 2, result.TotalParsed)

	first := result.Transactions[0]
	assert.Equal(t, "2026-10-01", first.Date)
	assert.Equal(t, "Miete", first.Payee)
	assert.Equal(t, 1234.56, first.Amount)
	assert.True(t, first.IsDebit)
	require.NotNil(t, first.Balance)
	assert.Equal(t, 1234.56, *first.Balance)

	second := result.Transactions[1]
	assert.Equal(t, "2026-10-02", second.Date)
	assert.Equal(t, "Gehalt", second.Payee)
	assert.Equal(t, 2500.0, second.Amount)
	assert.False(t, second.IsDebit)
	require.NotNil(t, second.Balance)
	assert.Equal(t, 3734.56, *second.Balance)
}

func TestIngestDuplicateRowsWithinStatement(t *testing.T) {
	db := openTestDB(t)

	content := `Date,Description,Debit,Credit
2026-09-15,Coffee Shop,150.00,
2026-09-15,Coffee Shop,150.00,
`
	result, err := Ingest(db, IngestionRequest{Format: FormatCSV, Content: content})
	require.NoError(t, err)
	require.Equal(t, 2, result.TotalParsed)
	assert.Equal(t, 1, result.PossibleDuplicates)

	assert.False(t, result.Transactions[0].IsDuplicate)
	assert.True(t, result.Transactions[1].IsDuplicate)
	assert.GreaterOrEqual(t, result.Transactions[1].DuplicateScore, 70)
	assert.Contains(t, result.Transactions[1].DuplicateReason, "same statement")
}

func TestIngestReadonlyDoesNotWriteManifest(t *testing.T) {
	db := openTestDB(t)
	root := useTempIngestConfigDir(t)

	require.NoError(t, config.LoadConfig([]byte(`
journal_path: main.ledger
db_path: paisa.db
default_currency: INR
readonly: true
`), ""))
	t.Cleanup(func() {
		_ = config.LoadConfig([]byte(`
journal_path: main.ledger
db_path: paisa.db
default_currency: INR
`), "")
	})

	content := `Date,Description,Debit,Credit
2026-09-15,Coffee Shop,150.00,
`
	result, err := Ingest(db, IngestionRequest{
		Format:       FormatCSV,
		Content:      content,
		SourceType:   "csv",
		OriginalPath: "checking.csv",
		Locale:       "en-IN",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, result.ImportID)
	assert.NoDirExists(t, filepath.Join(root, "imports"))
}

func TestIngestPersistsBalanceForReconciliation(t *testing.T) {
	db := openTestDB(t)
	root := useTempIngestConfigDir(t)

	content := `Date,Description,Debit,Credit,Balance
2026-09-15,Opening,,1000.00,1000.00
2026-09-16,Coffee Shop,150.00,,850.00
2026-09-17,Salary,,500.00,1350.00
`
	result, err := Ingest(db, IngestionRequest{
		Format:       FormatCSV,
		Content:      content,
		SourceType:   "csv",
		OriginalPath: "checking.csv",
		Locale:       "en-IN",
	})
	require.NoError(t, err)
	require.Len(t, result.Transactions, 3)
	for _, transaction := range result.Transactions {
		require.NotNil(t, transaction.Balance, "balance must be parsed from the CSV column")
	}

	recordsPath := filepath.Join(root, "imports", "csv", result.ImportID, "records.json")
	data, err := os.ReadFile(recordsPath)
	require.NoError(t, err)
	var records []StagedTransaction
	require.NoError(t, json.Unmarshal(data, &records))
	require.Len(t, records, 3)
	require.NotNil(t, records[0].Balance)
	assert.Equal(t, 1000.0, *records[0].Balance)

	report := reconciliation.BuildReconciliationFromManifests(root)
	require.Len(t, report.BankBalances, 1)
	check := report.BankBalances[0]
	assert.Equal(t, "ok", check.Status)
	assert.Equal(t, "ascending", check.Direction)
	require.NotNil(t, check.OpeningBalance)
	require.NotNil(t, check.ClosingBalance)
	assert.Equal(t, "0.00", *check.OpeningBalance)
	assert.Equal(t, "1350.00", *check.ClosingBalance)
}

func TestIngestComputesContentHashFromProvenance(t *testing.T) {
	db := openTestDB(t)
	useTempIngestConfigDir(t)

	content := `Date,Description,Debit,Credit
2026-09-15,Coffee Shop,150.00,
`
	result, err := Ingest(db, IngestionRequest{
		Format:       FormatCSV,
		Content:      content,
		SourceType:   "csv",
		OriginalPath: "checking.csv",
		Locale:       "en-IN",
	})
	require.NoError(t, err)
	assert.Equal(t,
		importmanifest.ImportID("csv", "checking.csv", sha256HexString(content)),
		result.ImportID,
	)
}
