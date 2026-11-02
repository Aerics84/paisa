package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ananthakumaran/paisa/internal/accounting"
	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/ledger"
	"github.com/ananthakumaran/paisa/internal/model/import_rule"
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openImportTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	accounting.ClearCache()
	t.Cleanup(accounting.ClearCache)

	name := strings.NewReplacer("/", "_", "\\", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&posting.Posting{}, &import_rule.Rule{}))
	return db
}

func loadImportTestConfig(t *testing.T, content string) {
	t.Helper()

	require.NoError(t, config.LoadConfig([]byte(content), ""))
	t.Cleanup(func() {
		_ = config.LoadConfig([]byte("journal_path: main.ledger\ndb_path: paisa.db\ndefault_currency: INR\nledger_cli: ledger\n"), "")
	})
}

func buildImportTestRouter(db *gorm.DB) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/import/statement", HandleParseStatement(db))
	r.POST("/api/import/commit", HandleCommitImport(db))
	r.GET("/api/rules", HandleGetRules(db))
	r.POST("/api/rules", HandleSaveRule(db))
	r.DELETE("/api/rules/:id", HandleDeleteRule(db))
	return r
}

func TestHandleParseStatement(t *testing.T) {
	db := openImportTestDB(t)
	loadImportTestConfig(t, "journal_path: main.ledger\ndb_path: paisa.db\ndefault_currency: INR\nledger_cli: ledger\n")
	router := buildImportTestRouter(db)

	require.NoError(t, import_rule.Save(db, &import_rule.Rule{
		Name:          "Groceries Rule",
		PayeePattern:  "Whole Foods",
		TargetAccount: "Expenses:Groceries",
		Priority:      10,
	}))

	payload := `{
		"format": "csv",
		"content": "Date,Description,Debit,Credit\n2026-10-01,Whole Foods Market,45.20,\n"
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/import/statement", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		TotalParsed  int `json:"total_parsed"`
		Transactions []struct {
			Payee           string `json:"payee"`
			SelectedAccount string `json:"selected_account"`
			MatchedRuleName string `json:"matched_rule_name"`
		} `json:"transactions"`
	}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, 1, resp.TotalParsed)
	require.Len(t, resp.Transactions, 1)
	assert.Equal(t, "Whole Foods Market", resp.Transactions[0].Payee)
	assert.Equal(t, "Expenses:Groceries", resp.Transactions[0].SelectedAccount)
	assert.Equal(t, "Groceries Rule", resp.Transactions[0].MatchedRuleName)
}

func TestRulesCRUD(t *testing.T) {
	db := openImportTestDB(t)
	loadImportTestConfig(t, "journal_path: main.ledger\ndb_path: paisa.db\ndefault_currency: INR\nledger_cli: ledger\n")
	router := buildImportTestRouter(db)

	rulePayload := `{
		"name": "Groceries Rule",
		"payee_pattern": "Whole Foods",
		"target_account": "Expenses:Groceries",
		"priority": 10
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/rules", bytes.NewBufferString(rulePayload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var saveResp struct {
		Rule  import_rule.Rule `json:"rule"`
		Saved bool             `json:"saved"`
	}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&saveResp))
	assert.True(t, saveResp.Saved)
	assert.NotZero(t, saveResp.Rule.ID)
	assert.Equal(t, "Groceries Rule", saveResp.Rule.Name)

	reqList := httptest.NewRequest(http.MethodGet, "/api/rules", nil)
	recList := httptest.NewRecorder()
	router.ServeHTTP(recList, reqList)
	require.Equal(t, http.StatusOK, recList.Code)

	var listResp struct {
		Rules []import_rule.Rule `json:"rules"`
	}
	require.NoError(t, json.NewDecoder(recList.Body).Decode(&listResp))
	require.Len(t, listResp.Rules, 1)
	assert.Equal(t, "Groceries Rule", listResp.Rules[0].Name)
	assert.Equal(t, "Expenses:Groceries", listResp.Rules[0].TargetAccount)

	reqDelete := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/rules/%d", saveResp.Rule.ID), nil)
	recDelete := httptest.NewRecorder()
	router.ServeHTTP(recDelete, reqDelete)
	require.Equal(t, http.StatusOK, recDelete.Code)

	var deleteResp struct {
		Success bool `json:"success"`
	}
	require.NoError(t, json.NewDecoder(recDelete.Body).Decode(&deleteResp))
	assert.True(t, deleteResp.Success)

	reqList = httptest.NewRequest(http.MethodGet, "/api/rules", nil)
	recList = httptest.NewRecorder()
	router.ServeHTTP(recList, reqList)
	require.Equal(t, http.StatusOK, recList.Code)
	listResp = struct {
		Rules []import_rule.Rule `json:"rules"`
	}{}
	require.NoError(t, json.NewDecoder(recList.Body).Decode(&listResp))
	assert.Empty(t, listResp.Rules)
}

func TestHandleCommitImport(t *testing.T) {
	db := openImportTestDB(t)

	journalPath := filepath.Join(t.TempDir(), "main.ledger")
	loadImportTestConfig(t, fmt.Sprintf(
		"journal_path: '%s'\ndb_path: paisa.db\ndefault_currency: INR\nledger_cli: ledger\n",
		filepath.ToSlash(journalPath),
	))
	router := buildImportTestRouter(db)
	stubImportValidation(t, nil)
	stubImportSyncSuccess(t)

	payload := `{
		"base_account": "Assets:Checking",
		"transactions": [
			{
				"date": "2026-10-11",
				"payee": "UPI-Smart Ba-14336279452-REF998877",
				"amount": 45.20,
				"currency": "INR",
				"is_debit": true,
				"selected_account": "Expenses:Food:Dining"
			}
		]
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/import/commit", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Success        bool   `json:"success"`
		CommittedCount int    `json:"committed_count"`
		TargetFile     string `json:"target_file"`
	}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.True(t, resp.Success)
	assert.Equal(t, 1, resp.CommittedCount)
	assert.Equal(t, filepath.ToSlash(journalPath), resp.TargetFile)

	content, err := os.ReadFile(journalPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "2026/10/11 * Smart Ba")
	assert.Contains(t, string(content), "Assets:Checking")
	assert.Contains(t, string(content), "-45.20 INR")
	assert.Contains(t, string(content), "Expenses:Food:Dining")
}

func TestHandleCommitImportValidatesCombinedBeancountJournal(t *testing.T) {
	db := openImportTestDB(t)

	journalPath := filepath.Join(t.TempDir(), "main.beancount")
	const existing = `option "title" "Test"

2000-01-01 open Assets:Checking
2000-01-01 open Expenses:Food:Dining
`
	seedImportJournal(t, journalPath, existing)
	loadImportTestConfig(t, fmt.Sprintf(
		"journal_path: '%s'\ndb_path: paisa.db\ndefault_currency: INR\nledger_cli: beancount\n",
		filepath.ToSlash(journalPath),
	))
	router := buildImportTestRouter(db)
	stubImportSyncSuccess(t)

	originalValidate := validateImportContent
	t.Cleanup(func() { validateImportContent = originalValidate })

	var validated string
	validateImportContent = func(content string) ([]ledger.LedgerFileError, string, error) {
		validated = content
		return nil, "", nil
	}

	payload := `{
		"base_account": "Assets:Checking",
		"transactions": [
			{
				"date": "2026-10-11",
				"payee": "Coffee Shop",
				"amount": 45.20,
				"currency": "INR",
				"is_debit": true,
				"selected_account": "Expenses:Food:Dining"
			}
		]
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/import/commit", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, validated, `open Assets:Checking`)
	assert.Contains(t, validated, `open Expenses:Food:Dining`)
	assert.Contains(t, validated, `2026-10-11 * "Coffee Shop"`)

	content, err := os.ReadFile(journalPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), `open Assets:Checking`)
	assert.Contains(t, string(content), `2026-10-11 * "Coffee Shop"`)
}

func TestHandleCommitImportRejectsEmpty(t *testing.T) {
	db := openImportTestDB(t)
	loadImportTestConfig(t, "journal_path: main.ledger\ndb_path: paisa.db\ndefault_currency: INR\nledger_cli: ledger\n")
	router := buildImportTestRouter(db)

	payload := `{"base_account": "Assets:Checking", "transactions": []}`
	req := httptest.NewRequest(http.MethodPost, "/api/import/commit", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "no transactions to commit")
}

func TestHandleCommitImportReadonly(t *testing.T) {
	db := openImportTestDB(t)

	journalPath := filepath.Join(t.TempDir(), "main.ledger")
	loadImportTestConfig(t, fmt.Sprintf(
		"journal_path: '%s'\ndb_path: paisa.db\ndefault_currency: INR\nledger_cli: ledger\nreadonly: true\n",
		filepath.ToSlash(journalPath),
	))
	router := buildImportTestRouter(db)

	payload := `{
		"base_account": "Assets:Checking",
		"transactions": [
			{
				"date": "2026-10-11",
				"payee": "Coffee Shop",
				"amount": 45.20,
				"currency": "INR",
				"is_debit": true,
				"selected_account": "Expenses:Food:Dining"
			}
		]
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/import/commit", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Success bool `json:"success"`
	}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.True(t, resp.Success)

	_, statErr := os.Stat(journalPath)
	assert.True(t, os.IsNotExist(statErr), "readonly commit must not create the journal file")
}

func stubImportValidation(t *testing.T, validationErr error) {
	t.Helper()
	original := validateImportContent
	t.Cleanup(func() { validateImportContent = original })
	validateImportContent = func(string) ([]ledger.LedgerFileError, string, error) {
		return nil, "", validationErr
	}
}

func stubImportSyncSuccess(t *testing.T) {
	t.Helper()
	original := importSync
	t.Cleanup(func() { importSync = original })
	importSync = func(*gorm.DB) gin.H {
		return gin.H{"success": true}
	}
}

func seedImportJournal(t *testing.T, path string, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
}

func assertImportJournalContent(t *testing.T, path string, expected string) {
	t.Helper()
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, expected, string(content))
}

func TestHandleCommitImportRejectsMissingBaseAccount(t *testing.T) {
	db := openImportTestDB(t)

	journalPath := filepath.Join(t.TempDir(), "main.ledger")
	const existing = "; existing journal\n"
	seedImportJournal(t, journalPath, existing)
	loadImportTestConfig(t, fmt.Sprintf(
		"journal_path: '%s'\ndb_path: paisa.db\ndefault_currency: INR\nledger_cli: ledger\n",
		filepath.ToSlash(journalPath),
	))
	router := buildImportTestRouter(db)

	payload := `{
		"transactions": [
			{
				"date": "2026-10-11",
				"payee": "Coffee Shop",
				"amount": 45.20,
				"currency": "INR",
				"is_debit": true,
				"selected_account": "Expenses:Food:Dining"
			}
		]
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/import/commit", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assertImportJournalContent(t, journalPath, existing)
}

func TestHandleCommitImportRejectsInvalidContent(t *testing.T) {
	db := openImportTestDB(t)

	journalPath := filepath.Join(t.TempDir(), "main.ledger")
	const existing = "; existing journal\n"
	seedImportJournal(t, journalPath, existing)
	loadImportTestConfig(t, fmt.Sprintf(
		"journal_path: '%s'\ndb_path: paisa.db\ndefault_currency: INR\nledger_cli: ledger\n",
		filepath.ToSlash(journalPath),
	))
	router := buildImportTestRouter(db)
	stubImportValidation(t, fmt.Errorf("invalid ledger syntax"))

	payload := `{
		"base_account": "Assets:Checking",
		"raw_content": "2026/10/11 * Unbalanced\n  Assets:Checking  100 INR\n"
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/import/commit", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assertImportJournalContent(t, journalPath, existing)
}

func TestHandleCommitImportRejectsNegativeAmount(t *testing.T) {
	db := openImportTestDB(t)

	journalPath := filepath.Join(t.TempDir(), "main.ledger")
	const existing = "; existing journal\n"
	seedImportJournal(t, journalPath, existing)
	loadImportTestConfig(t, fmt.Sprintf(
		"journal_path: '%s'\ndb_path: paisa.db\ndefault_currency: INR\nledger_cli: ledger\n",
		filepath.ToSlash(journalPath),
	))
	router := buildImportTestRouter(db)

	payload := `{
		"base_account": "Assets:Checking",
		"transactions": [
			{
				"date": "2026-10-11",
				"payee": "Coffee Shop",
				"amount": -45.20,
				"currency": "INR",
				"is_debit": true,
				"selected_account": "Expenses:Food:Dining"
			}
		]
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/import/commit", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assertImportJournalContent(t, journalPath, existing)
}

func TestHandleCommitImportRejectsNewlineInAccount(t *testing.T) {
	db := openImportTestDB(t)

	journalPath := filepath.Join(t.TempDir(), "main.ledger")
	const existing = "; existing journal\n"
	seedImportJournal(t, journalPath, existing)
	loadImportTestConfig(t, fmt.Sprintf(
		"journal_path: '%s'\ndb_path: paisa.db\ndefault_currency: INR\nledger_cli: ledger\n",
		filepath.ToSlash(journalPath),
	))
	router := buildImportTestRouter(db)

	payload := `{
		"base_account": "Assets:Checking",
		"transactions": [
			{
				"date": "2026-10-11",
				"payee": "Coffee Shop",
				"amount": 45.20,
				"currency": "INR",
				"is_debit": true,
				"selected_account": "Expenses:Food\nInjected"
			}
		]
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/import/commit", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assertImportJournalContent(t, journalPath, existing)
}

func TestHandleCommitImportRejectsInvalidDate(t *testing.T) {
	db := openImportTestDB(t)

	journalPath := filepath.Join(t.TempDir(), "main.ledger")
	const existing = "; existing journal\n"
	seedImportJournal(t, journalPath, existing)
	loadImportTestConfig(t, fmt.Sprintf(
		"journal_path: '%s'\ndb_path: paisa.db\ndefault_currency: INR\nledger_cli: ledger\n",
		filepath.ToSlash(journalPath),
	))
	router := buildImportTestRouter(db)

	payload := `{
		"base_account": "Assets:Checking",
		"transactions": [
			{
				"date": "11-10-2026",
				"payee": "Coffee Shop",
				"amount": 45.20,
				"currency": "INR",
				"is_debit": true,
				"selected_account": "Expenses:Food:Dining"
			}
		]
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/import/commit", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assertImportJournalContent(t, journalPath, existing)
}

func TestHandleCommitImportSyncsDatabase(t *testing.T) {
	db := openImportTestDB(t)

	journalPath := filepath.Join(t.TempDir(), "main.ledger")
	loadImportTestConfig(t, fmt.Sprintf(
		"journal_path: '%s'\ndb_path: paisa.db\ndefault_currency: INR\nledger_cli: ledger\n",
		filepath.ToSlash(journalPath),
	))
	router := buildImportTestRouter(db)
	stubImportValidation(t, nil)

	originalSync := importSync
	t.Cleanup(func() { importSync = originalSync })

	called := false
	importSync = func(syncDB *gorm.DB) gin.H {
		called = true
		assert.Same(t, db, syncDB)
		return gin.H{"success": true}
	}

	payload := `{
		"base_account": "Assets:Checking",
		"transactions": [
			{
				"date": "2026-10-11",
				"payee": "Coffee Shop",
				"amount": 45.20,
				"currency": "INR",
				"is_debit": true,
				"selected_account": "Expenses:Food:Dining"
			}
		]
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/import/commit", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, called)
}

func TestHandleCommitImportRollsBackWhenSyncFails(t *testing.T) {
	db := openImportTestDB(t)

	journalPath := filepath.Join(t.TempDir(), "main.ledger")
	const existing = "; existing journal\n"
	seedImportJournal(t, journalPath, existing)
	loadImportTestConfig(t, fmt.Sprintf(
		"journal_path: '%s'\ndb_path: paisa.db\ndefault_currency: INR\nledger_cli: ledger\n",
		filepath.ToSlash(journalPath),
	))
	router := buildImportTestRouter(db)
	stubImportValidation(t, nil)

	originalSync := importSync
	t.Cleanup(func() { importSync = originalSync })
	importSync = func(*gorm.DB) gin.H {
		return gin.H{"success": false, "message": "sync failed"}
	}

	payload := `{
		"base_account": "Assets:Checking",
		"transactions": [
			{
				"date": "2026-10-11",
				"payee": "Coffee Shop",
				"amount": 45.20,
				"currency": "INR",
				"is_debit": true,
				"selected_account": "Expenses:Food:Dining"
			}
		]
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/import/commit", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assertImportJournalContent(t, journalPath, existing)
}

func TestHandleParseStatementRejectsOversizedContent(t *testing.T) {
	db := openImportTestDB(t)
	loadImportTestConfig(t, "journal_path: main.ledger\ndb_path: paisa.db\ndefault_currency: INR\nledger_cli: ledger\n")
	router := buildImportTestRouter(db)

	payload := `{"format":"csv","content":"` + strings.Repeat("x", maxImportContentBytes+1) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/import/statement", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "too large")
}

func TestFormatTransactionsForCommitDialects(t *testing.T) {
	db := openImportTestDB(t)

	req := CommitImportRequest{
		BaseAccount: "Assets:Checking",
		Transactions: []CommitCandidateTransaction{
			{
				Date:            "2026-10-11",
				Payee:           "Coffee Shop",
				Amount:          45.20,
				Currency:        "INR",
				IsDebit:         true,
				SelectedAccount: "Expenses:Food:Dining",
			},
		},
	}

	loadImportTestConfig(t, "journal_path: main.ledger\ndb_path: paisa.db\ndefault_currency: INR\nledger_cli: ledger\n")
	ledgerOut := formatTransactionsForCommit(db, req, "INR")
	assert.Contains(t, ledgerOut, "2026/10/11 * Coffee Shop")
	assert.Contains(t, ledgerOut, "-45.20 INR")

	loadImportTestConfig(t, "journal_path: main.ledger\ndb_path: paisa.db\ndefault_currency: INR\nledger_cli: hledger\n")
	hledgerOut := formatTransactionsForCommit(db, req, "INR")
	assert.Contains(t, hledgerOut, "2026/10/11 * Coffee Shop")
	assert.Contains(t, hledgerOut, "-45.20 INR")

	loadImportTestConfig(t, "journal_path: main.ledger\ndb_path: paisa.db\ndefault_currency: INR\nledger_cli: beancount\n")
	beancountOut := formatTransactionsForCommit(db, req, "INR")
	assert.Contains(t, beancountOut, "2026-10-11 * \"Coffee Shop\"")
	assert.Contains(t, beancountOut, "-45.20 INR")
	assert.NotContains(t, beancountOut, "2026/10/11")
}
