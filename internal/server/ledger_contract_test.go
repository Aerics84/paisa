package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/ananthakumaran/paisa/internal/accounting"
	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/ananthakumaran/paisa/internal/utils"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

// TestLedgerJSONContract pins the wire format of GET /api/ledger. The fixture is
// checked in so that renaming a JSON field or changing a date from a string to
// an object cannot silently break the generated TypeScript types.
func TestLedgerJSONContract(t *testing.T) {
	utils.SetNow("2026-11-22")
	accounting.ClearCache()
	t.Cleanup(func() {
		utils.ResetNow()
		accounting.ClearCache()
	})

	require.NoError(t, config.LoadConfig([]byte("journal_path: global.ledger\ndb_path: global.db\n"), ""))

	store := config.NewStore()
	require.NoError(t, store.Load([]byte("journal_path: main.ledger\ndb_path: paisa.db\n"), ""))

	db := openTestDB(t)
	require.NoError(t, db.Create(ledgerContractPostings()).Error)

	router := Build(Deps{DB: db, Config: store}, false)

	req := httptest.NewRequest(http.MethodGet, "/api/ledger", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	expected, err := os.ReadFile("testdata/api/ledger.json")
	require.NoError(t, err)
	require.JSONEq(t, string(expected), rec.Body.String())
}

// The two postings form one balanced transaction, so the response also covers
// the balance accumulation of the ledger endpoint.
func ledgerContractPostings() []posting.Posting {
	return []posting.Posting{
		{
			ID:                   1,
			TransactionID:        "fixture-transaction",
			Date:                 time.Date(2026, 11, 22, 0, 0, 0, 0, time.UTC),
			Payee:                "Fixture Payee",
			Account:              "Assets:Checking:Fixture",
			Commodity:            "INR",
			Quantity:             decimal.NewFromInt(100),
			Amount:               decimal.NewFromInt(100),
			Status:               "cleared",
			TransactionBeginLine: 1,
			TransactionEndLine:   3,
			FileName:             "fixture.ledger",
			Note:                 "fixture note",
			TransactionNote:      "fixture transaction note",
		},
		{
			ID:                   2,
			TransactionID:        "fixture-transaction",
			Date:                 time.Date(2026, 11, 22, 0, 0, 0, 0, time.UTC),
			Payee:                "Fixture Payee",
			Account:              "Income:Salary",
			Commodity:            "INR",
			Quantity:             decimal.NewFromInt(-100),
			Amount:               decimal.NewFromInt(-100),
			Status:               "cleared",
			TransactionBeginLine: 1,
			TransactionEndLine:   3,
			FileName:             "fixture.ledger",
			TransactionNote:      "fixture transaction note",
		},
	}
}
