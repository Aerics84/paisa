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
	importmanifest "github.com/ananthakumaran/paisa/internal/importer/manifest"
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

func loadImportTestConfigAt(t *testing.T, configPath string, content string) {
	t.Helper()

	require.NoError(t, config.LoadConfig([]byte(content), configPath))
	t.Cleanup(func() {
		_ = config.LoadConfig([]byte("journal_path: main.ledger\ndb_path: paisa.db\ndefault_currency: INR\nledger_cli: ledger\n"), "")
	})
}

func buildImportTestRouter(db *gorm.DB) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/import/statement", HandleParseStatement(testDeps(db)))
	r.POST("/api/import/commit", HandleCommitImport(testDeps(db)))
	r.GET("/api/rules", HandleGetRules(db))
	r.POST("/api/rules", HandleSaveRule(testDeps(db)))
	r.DELETE("/api/rules/:id", HandleDeleteRule(testDeps(db)))
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
	body := rec.Body.Bytes()
	require.NoError(t, json.Unmarshal(body, &resp))
	assert.True(t, resp.Success)
	assert.Equal(t, 1, resp.CommittedCount)
	assert.Equal(t, filepath.ToSlash(journalPath), resp.TargetFile)

	// A clean commit keeps the historical null error list on the wire; the
	// generated TypeScript type reflects this as ImportValidationError[] | null.
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &raw))
	assert.Equal(t, "null", string(raw["errors"]))

	content, err := os.ReadFile(journalPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), `2026-10-11 * "Smart Ba"`)
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
	validateImportContent = func(_ *config.Store, content string) ([]ledger.LedgerFileError, string, error) {
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

func TestHandleCommitImportMarksManifestCommitted(t *testing.T) {
	db := openImportTestDB(t)

	journalPath := filepath.Join(t.TempDir(), "main.ledger")
	loadImportTestConfig(t, fmt.Sprintf(
		"journal_path: '%s'\ndb_path: paisa.db\ndefault_currency: INR\nledger_cli: ledger\n",
		filepath.ToSlash(journalPath),
	))
	router := buildImportTestRouter(db)
	stubImportValidation(t, nil)
	stubImportSyncSuccess(t)

	root := t.TempDir()
	originalRoot := importManifestRoot
	importManifestRoot = func(*config.Store) string { return root }
	t.Cleanup(func() { importManifestRoot = originalRoot })

	importID := strings.Repeat("a", 64)
	_, err := importmanifest.Write(root, importmanifest.Manifest{
		ImportID:      importID,
		SourceType:    "csv",
		OriginalPath:  "checking.csv",
		ContentSHA256: strings.Repeat("b", 64),
		ImportedAt:    "2026-10-01T10:00:00Z",
		Status:        "staged",
		Parser:        importmanifest.ParserInfo{Name: "paisa-import", Version: "1"},
	}, []any{})
	require.NoError(t, err)

	payload := fmt.Sprintf(`{
		"base_account": "Assets:Checking",
		"import_id": %q,
		"source_type": "csv",
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
	}`, importID)
	req := httptest.NewRequest(http.MethodPost, "/api/import/commit", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	manifests, err := importmanifest.ReadLatest(root, "csv")
	require.NoError(t, err)
	require.Len(t, manifests, 1)
	assert.Equal(t, "committed", manifests[0].Status)
}

func stubImportValidation(t *testing.T, validationErr error) {
	t.Helper()
	original := validateImportContent
	t.Cleanup(func() { validateImportContent = original })
	validateImportContent = func(*config.Store, string) ([]ledger.LedgerFileError, string, error) {
		return nil, "", validationErr
	}
}

func stubImportSyncSuccess(t *testing.T) {
	t.Helper()
	original := importSync
	t.Cleanup(func() { importSync = original })
	importSync = func(Deps) gin.H {
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
	importSync = func(deps Deps) gin.H {
		called = true
		assert.Same(t, db, deps.DB)
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
	importSync = func(Deps) gin.H {
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
	ledgerOut := formatTransactionsForCommit(db, req, "INR", config.DefaultStore())
	assert.Contains(t, ledgerOut, "2026-10-11 * \"Coffee Shop\"")
	assert.Contains(t, ledgerOut, "-45.20 INR")
	assert.NotContains(t, ledgerOut, "2026/10/11")

	loadImportTestConfig(t, "journal_path: main.ledger\ndb_path: paisa.db\ndefault_currency: INR\nledger_cli: hledger\n")
	hledgerOut := formatTransactionsForCommit(db, req, "INR", config.DefaultStore())
	assert.Contains(t, hledgerOut, "2026-10-11 * \"Coffee Shop\"")
	assert.Contains(t, hledgerOut, "-45.20 INR")
	assert.NotContains(t, hledgerOut, "2026/10/11")

	// German/EU journals declare `format 1.000,00 EUR`, so the decimal
	// separator must be a comma or hledger reads 45.20 as 4520.
	reqEUR := req
	reqEUR.Transactions = []CommitCandidateTransaction{{
		Date:            "2026-10-11",
		Payee:           "Coffee Shop",
		Amount:          45.20,
		Currency:        "EUR",
		IsDebit:         true,
		SelectedAccount: "Expenses:Food:Dining",
	}}
	loadImportTestConfig(t, "journal_path: main.ledger\ndb_path: paisa.db\nregional_profile: germany-eu\ndefault_currency: EUR\nlocale: de-DE\nledger_cli: hledger\n")
	germanOut := formatTransactionsForCommit(db, reqEUR, "EUR", config.DefaultStore())
	assert.Contains(t, germanOut, "-45,20 EUR")

	loadImportTestConfig(t, "journal_path: main.ledger\ndb_path: paisa.db\nregional_profile: germany-eu\ndefault_currency: EUR\nlocale: de-DE\nledger_cli: beancount\n")
	beancountGermanOut := formatTransactionsForCommit(db, reqEUR, "EUR", config.DefaultStore())
	assert.Contains(t, beancountGermanOut, "-45.20 EUR")

	loadImportTestConfig(t, "journal_path: main.ledger\ndb_path: paisa.db\ndefault_currency: INR\nledger_cli: beancount\n")
	beancountOut := formatTransactionsForCommit(db, req, "INR", config.DefaultStore())
	assert.Contains(t, beancountOut, "2026-10-11 * \"Coffee Shop\"")
	assert.Contains(t, beancountOut, "-45.20 INR")
	assert.NotContains(t, beancountOut, "2026/10/11")
}

func TestFormatTransactionsForCommitUsesJournalCommodityStyle(t *testing.T) {
	db := openImportTestDB(t)
	dir := t.TempDir()
	journal := filepath.Join(dir, "main.ledger")
	configPath := filepath.Join(dir, "paisa.yaml")

	req := CommitImportRequest{
		BaseAccount: "Assets:Checking",
		Transactions: []CommitCandidateTransaction{{
			Date:            "2026-10-11",
			Payee:           "Coffee Shop",
			Amount:          12.50,
			Currency:        "EUR",
			IsDebit:         true,
			SelectedAccount: "Expenses:Food:Dining",
		}},
	}

	// Relative journal_path must be resolved against the config directory, not
	// the process cwd. A dot-formatted journal keeps dots even when the
	// regional profile is Germany/EU; the journal style wins over the locale.
	require.NoError(t, os.WriteFile(journal, []byte("commodity EUR\n    format 1,000.00 EUR\n"), 0o644))
	loadImportTestConfigAt(t, configPath, "journal_path: main.ledger\ndb_path: paisa.db\nregional_profile: germany-eu\ndefault_currency: EUR\nlocale: de-DE\nledger_cli: hledger\n")
	dotOut := formatTransactionsForCommit(db, req, "EUR", config.DefaultStore())
	assert.Contains(t, dotOut, "-12.50 EUR")

	// A comma-formatted journal must use commas even without a German profile.
	require.NoError(t, os.WriteFile(journal, []byte("commodity EUR\n    format 1.000,00 EUR\n"), 0o644))
	loadImportTestConfigAt(t, configPath, "journal_path: main.ledger\ndb_path: paisa.db\ndefault_currency: EUR\nlocale: en\nledger_cli: hledger\n")
	commaOut := formatTransactionsForCommit(db, req, "EUR", config.DefaultStore())
	assert.Contains(t, commaOut, "-12,50 EUR")
}

func TestFormatTransactionsForCommitQuotesGroupedDebits(t *testing.T) {
	db := openImportTestDB(t)

	req := CommitImportRequest{
		BaseAccount: "Assets:Checking",
		GroupByDate: true,
		Transactions: []CommitCandidateTransaction{
			{
				Date:            "2026/10/11",
				Payee:           "Coffee Shop",
				Amount:          45.20,
				Currency:        "INR",
				IsDebit:         true,
				SelectedAccount: "Expenses:Food:Dining",
			},
			{
				Date:            "2026/10/11",
				Payee:           "Bakery & Café",
				Amount:          12.50,
				Currency:        "INR",
				IsDebit:         true,
				SelectedAccount: "Expenses:Food:Groceries",
			},
		},
	}

	loadImportTestConfig(t, "journal_path: main.ledger\ndb_path: paisa.db\ndefault_currency: INR\nledger_cli: ledger\n")
	out := formatTransactionsForCommit(db, req, "INR", config.DefaultStore())

	assert.Contains(t, out, `2026-10-11 * "Consolidated Debits (2 items)"`)
	assert.NotContains(t, out, "2026/10/11")
	assert.Contains(t, out, "Coffee Shop")
	assert.Contains(t, out, "Bakery Café")
}
