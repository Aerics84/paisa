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

	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

// TestEditorFilesJSONContract pins the wire format of GET /api/editor/files.
// The fixture is checked in so that renaming a JSON field or dropping the
// versions array cannot silently break the generated TypeScript types.
func TestEditorFilesJSONContract(t *testing.T) {
	base := t.TempDir()
	store := configureEditor(t, base, false)
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
	router := Build(Deps{DB: db, Config: store}, false)

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
	store := configureEditor(t, base, false)
	writeEditorFile(t, filepath.Join(base, "budget.paisa"), "value = 1\n")

	db := openTestDB(t)
	router := Build(Deps{DB: db, Config: store}, false)

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
	store := configureEditor(t, base, false)

	db := openTestDB(t)
	router := Build(Deps{DB: db, Config: store}, false)

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

// TestEditorSaveValidationErrorJSONContract pins a populated FileError list.
// A traversal name fails validation before any ledger binary is invoked, so
// this deterministically exercises fileErrors plus the file_name omitempty tag
// and the error/message keys.
func TestEditorSaveValidationErrorJSONContract(t *testing.T) {
	base := t.TempDir()
	store := configureEditor(t, base, false)

	db := openTestDB(t)
	router := Build(Deps{DB: db, Config: store}, false)

	payload := `{"name": "../outside.ledger", "content": ""}`
	req := httptest.NewRequest(http.MethodPost, "/api/editor/save", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var got EditorSaveResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.False(t, got.Saved)
	require.Len(t, got.Errors, 1)
	require.Empty(t, got.Errors[0].FileName)
	require.Equal(t, uint64(1), got.Errors[0].LineFrom)
	require.Equal(t, uint64(1), got.Errors[0].LineTo)
	require.Equal(t, "", got.Errors[0].Error)
	require.Equal(t, "Invalid file name", got.Errors[0].Message)
	require.Nil(t, got.File)
	require.Equal(t, "Validation failed", got.Message)

	expected, err := os.ReadFile("testdata/api/editor_save_validation_error.json")
	require.NoError(t, err)
	require.JSONEq(t, string(expected), rec.Body.String())

	// The validate endpoint uses the same conversion; pin its full body too.
	validateReq := httptest.NewRequest(http.MethodPost, "/api/editor/validate", bytes.NewBufferString(payload))
	validateReq.Header.Set("Content-Type", "application/json")
	validateRec := httptest.NewRecorder()
	router.ServeHTTP(validateRec, validateReq)
	require.Equal(t, http.StatusOK, validateRec.Code)
	require.JSONEq(
		t,
		`{"errors":[{"line_from":1,"line_to":1,"error":"","message":"Invalid file name"}],"output":""}`,
		validateRec.Body.String(),
	)
}

// TestEditorAndSheetResponsesHaveNoNullOnEmptyDB enforces the null discipline:
// list responses and file responses use empty arrays/objects instead of null.
func TestEditorAndSheetResponsesHaveNoNullOnEmptyDB(t *testing.T) {
	base := t.TempDir()
	store := configureEditor(t, base, false)
	writeEditorFile(t, filepath.Join(base, "main.ledger"), "2026-11-01 Fixture\n")
	writeEditorFile(t, filepath.Join(base, "budget.paisa"), "value = 1\n")

	db := openTestDB(t)
	router := Build(Deps{DB: db, Config: store}, false)

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

func TestEditorFileNamesUseForwardSlashes(t *testing.T) {
	base := t.TempDir()
	path := filepath.Join(base, "archive", "old.ledger")
	writeEditorFile(t, path, "2026-11-01 Nested\n")
	writeEditorFile(t, path+".backup.2026-11-20-10-00-00.000", "2026-10-01 Old\n")

	file := readLedgerFileWithVersions(base, path)
	require.Equal(t, "archive/old.ledger", file.Name)
	require.Equal(t, []string{"archive/old.ledger.backup.2026-11-20-10-00-00.000"}, file.Versions)
}
