package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/utils"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestConfigResponseShape(t *testing.T) {
	utils.ResetNow()
	t.Cleanup(utils.ResetNow)

	router := newConfigTestRouter(t)
	payload := requestConfig(t, router)

	var body ConfigResponse
	require.NoError(t, json.Unmarshal(payload, &body))
	require.NotNil(t, body.Schema)
	require.Equal(t, "main.ledger", body.Config.JournalPath)
	require.Nil(t, body.Now)

	// A nil Now must be omitted, not serialized as null: the generated
	// TypeScript type is `now?: string`.
	var raw map[string]any
	require.NoError(t, json.Unmarshal(payload, &raw))
	require.NotContains(t, raw, "now")
	require.ElementsMatch(t, []string{"config", "accounts", "schema"}, mapKeys(raw))
}

func TestConfigResponseNowUsesRFC3339(t *testing.T) {
	utils.SetNow("2026-11-22")
	t.Cleanup(utils.ResetNow)

	router := newConfigTestRouter(t)
	payload := requestConfig(t, router)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(payload, &raw))
	require.Contains(t, raw, "now")
	require.Equal(t, "2026-11-22T00:00:00Z", raw["now"])
	require.ElementsMatch(t, []string{"config", "accounts", "now", "schema"}, mapKeys(raw))
}

func newConfigTestRouter(t *testing.T) *gin.Engine {
	t.Helper()

	// The auth middleware reads the process-wide store. Other tests in this
	// package load configurations with user accounts, so make sure the global
	// configuration is userless before hitting /api/config without credentials.
	require.NoError(t, config.LoadConfig([]byte("journal_path: global.ledger\ndb_path: global.db\n"), ""))

	store := config.NewStore()
	require.NoError(t, store.Load([]byte("journal_path: main.ledger\ndb_path: paisa.db\n"), ""))

	return Build(Deps{DB: openTestDB(t), Config: store}, false)
}

func requestConfig(t *testing.T, router *gin.Engine) []byte {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	return rec.Body.Bytes()
}

func mapKeys(values map[string]any) []string {
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
