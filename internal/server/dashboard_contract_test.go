package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/stretchr/testify/require"
)

// TestDashboardJSONShapeOnEmptyDB pins the null-vs-empty-array contract of
// GET /api/dashboard. The generated TypeScript types declare arrays, maps and
// optional objects, so the JSON body must not use null for them.
func TestDashboardJSONShapeOnEmptyDB(t *testing.T) {
	// The auth middleware reads the process-wide store; keep it userless so the
	// request is not rejected.
	require.NoError(t, config.LoadConfig([]byte("journal_path: global.ledger\ndb_path: global.db\n"), ""))

	store := config.NewStore()
	require.NoError(t, store.Load([]byte("journal_path: main.ledger\ndb_path: paisa.db\n"), ""))

	router := Build(Deps{DB: openTestDB(t), Config: store}, false)

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))

	for _, key := range []string{
		"checkingBalanceTimeline",
		"cashFlows",
		"transactionSequences",
		"transactions",
		"goalSummaries",
	} {
		require.IsType(t, []any{}, body[key], "%s must be a JSON array", key)
	}

	for _, key := range []string{"expenses"} {
		require.IsType(t, map[string]any{}, body[key], "%s must be a JSON object", key)
	}

	checkingBalances, ok := body["checkingBalances"].(map[string]any)
	require.True(t, ok)
	require.IsType(t, map[string]any{}, checkingBalances["asset_breakdowns"])

	budget, ok := body["budget"].(map[string]any)
	require.True(t, ok)
	require.IsType(t, map[string]any{}, budget["budgetsByMonth"])

	// expense_pace is omitted (not null) when there is no projection.
	require.NotContains(t, body, "expense_pace")

	assertNoNull(t, body, "dashboard")
}

func assertNoNull(t *testing.T, value any, path string) {
	t.Helper()

	switch typed := value.(type) {
	case nil:
		t.Errorf("%s must not be null", path)
	case []any:
		for index, item := range typed {
			assertNoNull(t, item, fmt.Sprintf("%s[%d]", path, index))
		}
	case map[string]any:
		for key, item := range typed {
			assertNoNull(t, item, path+"."+key)
		}
	}
}
