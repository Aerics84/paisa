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
	"github.com/ananthakumaran/paisa/internal/importer/manifest"
	"github.com/ananthakumaran/paisa/internal/model/import_rule"
	"github.com/stretchr/testify/require"
)

// TestImportReconciliationJSONContract pins the wire format of
// GET /api/import/reconciliation. The fixture is checked in so renaming a JSON
// field or changing a balance from a string to a number cannot silently break
// the generated TypeScript types.
func TestImportReconciliationJSONContract(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "paisa.yaml")
	store := config.NewStore()
	require.NoError(t, store.Load([]byte(
		"journal_path: main.ledger\ndb_path: paisa.db\ndefault_currency: INR\nledger_cli: ledger\n",
	), configPath))

	importID := manifest.ImportID("csv", "fixtures/bank.csv", "fixture-content-hash")
	balance1000 := 1100.00
	balance1050 := 1050.00
	balance1075 := 1075.50
	amount100 := 100.0
	amount50 := 50.0
	amount25 := 25.5
	_, err := manifest.Write(dir, manifest.Manifest{
		ImportID:      importID,
		SourceType:    "csv",
		OriginalPath:  "fixtures/bank.csv",
		ContentSHA256: "fixture-content-hash",
		ImportedAt:    "2026-11-22T00:00:00Z",
		Status:        "committed",
	}, []map[string]any{
		{"date": "2026-11-01", "payee": "Opening", "amount": amount100, "currency": "INR", "is_debit": false, "balance": balance1000},
		{"date": "2026-11-02", "payee": "Groceries", "amount": amount50, "currency": "INR", "is_debit": true, "balance": balance1050},
		{"date": "2026-11-03", "payee": "Refund", "amount": amount25, "currency": "INR", "is_debit": false, "balance": balance1075},
	})
	require.NoError(t, err)

	db := openImportTestDB(t)
	router := Build(Deps{DB: db, Config: store}, false)

	req := httptest.NewRequest(http.MethodGet, "/api/import/reconciliation", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var got ReconciliationResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))

	expected, err := os.ReadFile("testdata/api/import_reconciliation.json")
	require.NoError(t, err)
	require.JSONEq(t, normalizeGeneratedAt(t, string(expected)), normalizeGeneratedAt(t, rec.Body.String()))
}

// TestRulesJSONContract pins the wire format of GET /api/rules.
func TestRulesJSONContract(t *testing.T) {
	db := openImportTestDB(t)
	loadImportTestConfig(t, "journal_path: main.ledger\ndb_path: paisa.db\ndefault_currency: INR\nledger_cli: ledger\n")

	minAmount := 10.0
	maxAmount := 500.0
	require.NoError(t, import_rule.Save(db, &import_rule.Rule{
		Name:           "Fixture Groceries",
		Priority:       10,
		Enabled:        true,
		PayeePattern:   "Whole Foods",
		PayeeMatchType: import_rule.MatchContains,
		MemoPattern:    "",
		MemoMatchType:  import_rule.MatchContains,
		TxType:         import_rule.TxTypeAny,
		MinAmount:      &minAmount,
		MaxAmount:      &maxAmount,
		TargetAccount:  "Expenses:Groceries",
		Tags:           []string{"food"},
		FlagForReview:  false,
	}))

	router := Build(Deps{DB: db, Config: config.NewStore()}, false)

	req := httptest.NewRequest(http.MethodGet, "/api/rules", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var got RulesResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Len(t, got.Rules, 1)

	expected, err := os.ReadFile("testdata/api/rules.json")
	require.NoError(t, err)
	require.JSONEq(t, string(expected), rec.Body.String())
}

// TestTemplatesJSONContract pins the success shape of a single template
// upsert. The GET list is checked structurally instead of as a golden file
// because it embeds the large set of builtin handlebars templates, which would
// make the fixture churn on every bundled template change.
func TestTemplatesJSONContract(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "paisa.yaml")
	require.NoError(t, config.LoadConfig([]byte(
		"journal_path: main.ledger\ndb_path: paisa.db\ndefault_currency: INR\nledger_cli: ledger\n"+
			"import_templates:\n  - name: Existing\n    content: \"{{date}}\"\n",
	), configPath))
	t.Cleanup(func() {
		_ = config.LoadConfig([]byte("journal_path: main.ledger\ndb_path: paisa.db\ndefault_currency: INR\nledger_cli: ledger\n"), "")
	})

	db := openImportTestDB(t)
	router := Build(Deps{DB: db, Config: config.NewStore()}, false)

	payload := `{"name": "Fixture Custom", "content": "{{payee}}"}`
	req := httptest.NewRequest(http.MethodPost, "/api/templates/upsert", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	expected, err := os.ReadFile("testdata/api/template_upsert.json")
	require.NoError(t, err)
	require.JSONEq(t, string(expected), rec.Body.String())

	// GET /api/templates also includes builtin templates. Assert the custom
	// template we just wrote is present with the documented field set instead
	// of pinning the embedded list.
	listReq := httptest.NewRequest(http.MethodGet, "/api/templates", nil)
	listRec := httptest.NewRecorder()
	router.ServeHTTP(listRec, listReq)
	require.Equal(t, http.StatusOK, listRec.Code)

	var list TemplatesResponse
	require.NoError(t, json.Unmarshal(listRec.Body.Bytes(), &list))

	var found bool
	for _, template := range list.Templates {
		require.NotEmpty(t, template.ID)
		require.NotEmpty(t, template.Name)
		require.NotEmpty(t, template.TemplateType)
		if template.Name == "Fixture Custom" {
			found = true
			require.Equal(t, "custom:Fixture Custom", template.ID)
			require.Equal(t, "{{payee}}", template.Content)
			require.Equal(t, "custom", string(template.TemplateType))
		}
	}
	require.True(t, found, "upserted template must appear in GET /api/templates")
}

// normalizeGeneratedAt replaces the wall-clock timestamp of a reconciliation
// report with a placeholder. The rest of the report is pinned byte-for-byte.
func normalizeGeneratedAt(t *testing.T, body string) string {
	t.Helper()

	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(body), &payload))

	generatedAt, ok := payload["generated_at"].(string)
	require.True(t, ok, "generated_at must be a string")
	if generatedAt != "GENERATED_AT" {
		_, err := time.Parse(time.RFC3339, generatedAt)
		require.NoError(t, err, "generated_at must be RFC3339")
	}

	payload["generated_at"] = "GENERATED_AT"
	normalized, err := json.Marshal(payload)
	require.NoError(t, err)
	return string(normalized)
}
