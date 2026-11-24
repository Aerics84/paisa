package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestConfigResponseShape(t *testing.T) {
	// The auth middleware reads the process-wide store. Other tests in this
	// package load configurations with user accounts, so make sure the global
	// configuration is userless before hitting /api/config without credentials.
	require.NoError(t, config.LoadConfig([]byte("journal_path: global.ledger\ndb_path: global.db\n"), ""))

	store := config.NewStore()
	require.NoError(t, store.Load([]byte("journal_path: main.ledger\ndb_path: paisa.db\n"), ""))
	router := Build(Deps{DB: openTestDB(t), Config: store}, false)

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var body ConfigResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.NotNil(t, body.Schema)
	require.Equal(t, "main.ledger", body.Config.JournalPath)

	// The JSON keys must stay exactly the same as the previous gin.H payload.
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &raw))
	require.ElementsMatch(t, []string{"config", "accounts", "now", "schema"}, mapKeys(raw))
}

func mapKeys(values map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}

func TestAPIWireFormatUsesStringsAndNumbers(t *testing.T) {
	type sample struct {
		At     time.Time       `json:"at"`
		Amount decimal.Decimal `json:"amount"`
	}
	body, err := json.Marshal(sample{
		At:     time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC),
		Amount: decimal.NewFromInt(42),
	})
	require.NoError(t, err)
	require.Contains(t, string(body), `"at":"2026-11-22T12:00:00Z"`)
	require.Contains(t, string(body), `"amount":42`)
}
