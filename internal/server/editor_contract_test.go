package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

// TestEditorFilesJSONContract pins the wire format of GET /api/editor/files.
// The fixture is checked in so that renaming a JSON field or dropping the
// versions array cannot silently break the generated TypeScript types.
func TestEditorFilesJSONContract(t *testing.T) {
	base := t.TempDir()
	configureEditor(t, base, false)
	writeEditorFile(t, filepath.Join(base, "main.ledger"), "2026-11-01 Fixture\n")
	writeEditorFile(t, filepath.Join(base, "main.ledger.backup.2026-11-20-10-00-00.000"), "2026-10-01 Old\n")

	db := openTestDB(t)
	require.NoError(t, db.Create(&posting.Posting{
		ID:                   1,
		TransactionID:        "fixture-transaction",
		Date:                 time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC),
		Payee:                "Fixture Payee",
		Account:              "Assets:Checking",
		Commodity:            "INR",
		Quantity:             decimal.NewFromInt(100),
		Amount:               decimal.NewFromInt(100),
		Status:               "cleared",
		TransactionBeginLine: 1,
		TransactionEndLine:   2,
		FileName:             "main.ledger",
	}).Error)
	router := Build(Deps{DB: db, Config: config.NewStore()}, false)

	req := httptest.NewRequest(http.MethodGet, "/api/editor/files", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var got EditorFilesResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Len(t, got.Files, 1)

	expected, err := os.ReadFile("testdata/api/editor_files.json")
	require.NoError(t, err)
	require.JSONEq(t, string(expected), rec.Body.String())
}

// TestSheetFilesJSONContract pins the wire format of GET /api/sheets/files.
// The empty posting list is intentional: it exercises the nil-slice
// normalization that keeps the array non-null on an empty database.
func TestSheetFilesJSONContract(t *testing.T) {
	base := t.TempDir()
	configureEditor(t, base, false)
	writeEditorFile(t, filepath.Join(base, "budget.paisa"), "value = 1\n")

	db := openTestDB(t)
	router := Build(Deps{DB: db, Config: config.NewStore()}, false)

	req := httptest.NewRequest(http.MethodGet, "/api/sheets/files", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var got SheetFilesResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Len(t, got.Files, 1)

	expected, err := os.ReadFile("testdata/api/sheet_files.json")
	require.NoError(t, err)
	require.JSONEq(t, string(expected), rec.Body.String())
}

// TestEditorSaveJSONContract pins a failed save response. A successful save
// runs the external ledger validator, which is not available in unit tests, but
// the extension check fails before validation and exercises the same
// {errors, saved, message} envelope.
func TestEditorSaveJSONContract(t *testing.T) {
	base := t.TempDir()
	configureEditor(t, base, false)

	db := openTestDB(t)
	router := Build(Deps{DB: db, Config: config.NewStore()}, false)

	payload := `{"name": "expenses.txt", "content": ""}`
	req := httptest.NewRequest(http.MethodPost, "/api/editor/save", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var got EditorSaveResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.False(t, got.Saved)
	require.Len(t, got.Errors, 0)
	require.Nil(t, got.File)
	require.Equal(t, "File name must end with .ledger", got.Message)

	expected, err := os.ReadFile("testdata/api/editor_save.json")
	require.NoError(t, err)
	require.JSONEq(t, string(expected), rec.Body.String())
}

// TestEditorAndSheetResponsesHaveNoNullOnEmptyDB enforces the null discipline:
// list responses and file responses use empty arrays/objects instead of null.
func TestEditorAndSheetResponsesHaveNoNullOnEmptyDB(t *testing.T) {
	base := t.TempDir()
	configureEditor(t, base, false)
	writeEditorFile(t, filepath.Join(base, "main.ledger"), "2026-11-01 Fixture\n")
	writeEditorFile(t, filepath.Join(base, "budget.paisa"), "value = 1\n")

	db := openTestDB(t)
	router := Build(Deps{DB: db, Config: config.NewStore()}, false)

	requests := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/api/editor/files", ""},
		{http.MethodGet, "/api/sheets/files", ""},
		{http.MethodPost, "/api/editor/file", `{"name": "main.ledger"}`},
		{http.MethodPost, "/api/sheets/file", `{"name": "budget.paisa"}`},
		{http.MethodPost, "/api/editor/validate", `{"name": "main.ledger", "content": "2026-11-01 Fixture\n"}`},
	}

	for _, request := range requests {
		t.Run(request.method+" "+request.path, func(t *testing.T) {
			var body *bytes.Buffer
			if request.body == "" {
				body = bytes.NewBuffer(nil)
			} else {
				body = bytes.NewBufferString(request.body)
			}
			req := httptest.NewRequest(request.method, request.path, body)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			require.Equal(t, http.StatusOK, rec.Code)
			require.NotContains(t, rec.Body.String(), "null", "response must not contain null values")
		})
	}
}
