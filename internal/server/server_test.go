package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRateLimitKeyUsesClientIP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/ping", nil)
	ctx.Request.RemoteAddr = "203.0.113.9:1234"
	assert.Equal(t, "203.0.113.9", rateLimitKey(ctx))
}

func TestBuildUsesInjectedConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// The auth middleware reads the process-wide store. Other tests in this
	// package load configurations with user accounts, so make sure the global
	// configuration is userless before hitting /api/config without credentials.
	require.NoError(t, config.LoadConfig([]byte("journal_path: global.ledger\ndb_path: global.db\ndefault_currency: INR\n"), ""))

	db := openTestDB(t)

	dir := t.TempDir()
	storeA := config.NewStore()
	storeB := config.NewStore()
	require.NoError(t, storeA.Load([]byte("journal_path: a.ledger\ndb_path: a.db\ndefault_currency: EUR\n"), filepath.Join(dir, "a.yaml")))
	require.NoError(t, storeB.Load([]byte("journal_path: b.ledger\ndb_path: b.db\ndefault_currency: INR\n"), filepath.Join(dir, "b.yaml")))

	routerA := Build(Deps{DB: db, Config: storeA}, false)
	routerB := Build(Deps{DB: db, Config: storeB}, false)

	require.Equal(t, "EUR", configFromRouter(t, routerA))
	require.Equal(t, "INR", configFromRouter(t, routerB))
}

// testDeps builds handler dependencies around a database. The configuration is
// the process-wide store because the helpers in these tests load their fixture
// through config.LoadConfig; the injected-config behaviour is covered by
// TestNetworthHandlerUsesInjectedConfig.
func testDeps(db *gorm.DB) Deps {
	return Deps{DB: db, Config: config.DefaultStore()}
}

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	name := strings.NewReplacer("/", "_", "\\", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&posting.Posting{}))

	return db
}

func configFromRouter(t *testing.T, router *gin.Engine) string {
	t.Helper()

	request := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code)

	var body struct {
		Config struct {
			DefaultCurrency string `json:"default_currency"`
		} `json:"config"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))

	return body.Config.DefaultCurrency
}
