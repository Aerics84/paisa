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
	"github.com/ananthakumaran/paisa/internal/model/transaction"
	"github.com/ananthakumaran/paisa/internal/service"
	"github.com/ananthakumaran/paisa/internal/utils"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const liabilitiesContractConfig = `journal_path: main.ledger
db_path: paisa.db
default_currency: EUR
time_zone: UTC
financial_year_starting_month: 1
credit_cards:
  - account: Liabilities:CreditCard:Visa
    credit_limit: 100000
    statement_end_day: 20
    due_day: 5
    network: visa
    number: "4111"
    expiration_date: "2030-12-31"
`

func openLiabilitiesContractDB(t *testing.T) *gorm.DB {
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
	transaction.ClearCache()
	t.Cleanup(service.ClearInterestCache)
	t.Cleanup(service.ClearPriceCache)
	t.Cleanup(accounting.ClearCache)
	t.Cleanup(transaction.ClearCache)

	return db
}

func liabilitiesContractRouter(t *testing.T, db *gorm.DB) *ginEngine {
	t.Helper()

	require.NoError(t, config.LoadConfig([]byte(liabilitiesContractConfig), ""))
	store := config.NewStore()
	require.NoError(t, store.Load([]byte(liabilitiesContractConfig), ""))

	return &ginEngine{router: Build(Deps{DB: db, Config: store}, false)}
}

// liabilitiesContractPostings is a stable fixture with one loan, one interest
// payment and one credit card statement containing a spend and a payment.
func liabilitiesContractPostings() []posting.Posting {
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
		mk(1, "fixture-loan", "2026-11-20", "Liabilities:Loan", -1000),
		mk(2, "fixture-repay", "2026-11-21", "Liabilities:Loan", 200),
		mk(3, "fixture-interest", "2026-11-21", "Expenses:Interest:Loan", 10),
		mk(4, "fixture-card-spend", "2026-11-19", "Liabilities:CreditCard:Visa", -500),
		mk(5, "fixture-card-pay", "2026-11-20", "Liabilities:CreditCard:Visa", 100),
	}
}

// TestLiabilityInterestJSONContract pins the wire format of
// GET /api/liabilities/interest, including the nested overview timeline.
func TestLiabilityInterestJSONContract(t *testing.T) {
	utils.SetNow("2026-11-22")
	t.Cleanup(utils.ResetNow)

	db := openLiabilitiesContractDB(t)
	require.NoError(t, db.Create(liabilitiesContractPostings()).Error)
	engine := liabilitiesContractRouter(t, db)

	rec := engine.get(t, "/api/liabilities/interest")
	require.Equal(t, http.StatusOK, rec.Code)

	var body LiabilityInterestResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))

	expected, err := os.ReadFile("testdata/api/liabilities_interest.json")
	require.NoError(t, err)
	require.JSONEq(t, string(expected), rec.Body.String())
}

// TestLiabilityBalanceJSONContract pins the group-keyed liability breakdowns.
func TestLiabilityBalanceJSONContract(t *testing.T) {
	utils.SetNow("2026-11-22")
	t.Cleanup(utils.ResetNow)

	db := openLiabilitiesContractDB(t)
	require.NoError(t, db.Create(liabilitiesContractPostings()).Error)
	engine := liabilitiesContractRouter(t, db)

	rec := engine.get(t, "/api/liabilities/balance")
	require.Equal(t, http.StatusOK, rec.Code)

	var body LiabilityBalanceResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))

	expected, err := os.ReadFile("testdata/api/liabilities_balance.json")
	require.NoError(t, err)
	require.JSONEq(t, string(expected), rec.Body.String())
}

// TestLiabilityRepaymentJSONContract pins the repayment posting list.
func TestLiabilityRepaymentJSONContract(t *testing.T) {
	utils.SetNow("2026-11-22")
	t.Cleanup(utils.ResetNow)

	db := openLiabilitiesContractDB(t)
	require.NoError(t, db.Create(liabilitiesContractPostings()).Error)
	engine := liabilitiesContractRouter(t, db)

	rec := engine.get(t, "/api/liabilities/repayment")
	require.Equal(t, http.StatusOK, rec.Code)

	var body LiabilityRepaymentResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))

	expected, err := os.ReadFile("testdata/api/liabilities_repayment.json")
	require.NoError(t, err)
	require.JSONEq(t, string(expected), rec.Body.String())
}

// TestCreditCardsJSONContract pins the credit card summary list, including the
// bill dates and the nullable paidDate of an outstanding bill.
func TestCreditCardsJSONContract(t *testing.T) {
	utils.SetNow("2026-11-22")
	t.Cleanup(utils.ResetNow)

	db := openLiabilitiesContractDB(t)
	require.NoError(t, db.Create(liabilitiesContractPostings()).Error)
	engine := liabilitiesContractRouter(t, db)

	rec := engine.get(t, "/api/credit_cards")
	require.Equal(t, http.StatusOK, rec.Code)

	var body CreditCardsResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))

	expected, err := os.ReadFile("testdata/api/credit_cards.json")
	require.NoError(t, err)
	require.JSONEq(t, string(expected), rec.Body.String())
}

// TestCreditCardJSONContract pins the single credit card payload with postings
// and transactions, plus the not-found body.
func TestCreditCardJSONContract(t *testing.T) {
	utils.SetNow("2026-11-22")
	t.Cleanup(utils.ResetNow)

	db := openLiabilitiesContractDB(t)
	require.NoError(t, db.Create(liabilitiesContractPostings()).Error)
	engine := liabilitiesContractRouter(t, db)

	rec := engine.get(t, "/api/credit_cards/Liabilities:CreditCard:Visa")
	require.Equal(t, http.StatusOK, rec.Code)

	var body CreditCardResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))

	expected, err := os.ReadFile("testdata/api/credit_card.json")
	require.NoError(t, err)
	require.JSONEq(t, string(expected), rec.Body.String())

	notFound := engine.get(t, "/api/credit_cards/Missing")
	require.Equal(t, http.StatusOK, notFound.Code)
	require.JSONEq(t, `{"found": false}`, notFound.Body.String())
}

// TestLiabilityResponsesHaveNoNullOnEmptyDB pins the null-vs-empty contract of
// the liabilities and credit card endpoints. The generated TypeScript types
// declare arrays and maps, so an empty journal must not produce JSON null.
func TestLiabilityResponsesHaveNoNullOnEmptyDB(t *testing.T) {
	utils.SetNow("2026-11-22")
	t.Cleanup(utils.ResetNow)

	db := openLiabilitiesContractDB(t)
	engine := liabilitiesContractRouter(t, db)

	for _, path := range []string{
		"/api/liabilities/interest",
		"/api/liabilities/balance",
		"/api/liabilities/repayment",
		"/api/credit_cards",
		"/api/credit_cards/Missing",
	} {
		rec := engine.get(t, path)
		require.Equal(t, http.StatusOK, rec.Code, path)

		var body map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), path)
		assertNoNull(t, body, path)
	}
}
