package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ananthakumaran/paisa/internal/auth"
	"github.com/ananthakumaran/paisa/internal/config"
	modelcache "github.com/ananthakumaran/paisa/internal/model/cache"
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/ananthakumaran/paisa/internal/model/price"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const onboardingLoopback = "127.0.0.1:54321"

const onboardingBaseConfig = `
journal_path: main.ledger
db_path: paisa.db
regional_profile: germany-eu
`

// newOnboardingEnv binds the process-wide store to an isolated configuration
// file. The onboarding handler runs the existing sync path, which reads the
// journal path from the process-wide store.
func newOnboardingEnv(t *testing.T, yaml string, autoCreated bool) (*config.Store, string, string) {
	t.Helper()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "paisa.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(yaml), 0o644))
	require.NoError(t, config.LoadConfig([]byte(yaml), configPath))

	store := config.DefaultStore()
	// Clear state left behind by other tests in this package.
	store.MarkInitialized()
	if autoCreated {
		store.MarkAutoCreated()
	}

	return store, dir, configPath
}

func openOnboardingTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	name := strings.NewReplacer("/", "_", "\\", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&posting.Posting{}, &price.Price{}, &modelcache.Cache{}))

	return db
}

func newOnboardingRouter(t *testing.T, db *gorm.DB, store *config.Store) *gin.Engine {
	t.Helper()

	return Build(Deps{DB: db, Config: store, Auth: auth.NewStore(time.Hour)}, false)
}

func getOnboarding(t *testing.T, router *gin.Engine) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()

	request := httptest.NewRequest(http.MethodGet, "/api/onboarding", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	body := map[string]any{}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))

	return recorder, body
}

func postOnboarding(t *testing.T, router *gin.Engine, body string, remoteAddr string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(http.MethodPost, "/api/onboarding", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.RemoteAddr = remoteAddr
	for key, value := range headers {
		request.Header.Set(key, value)
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestOnboardingExistingEmptyJournalDoesNotNeedOnboarding(t *testing.T) {
	store, dir, _ := newOnboardingEnv(t, onboardingBaseConfig, false)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.ledger"), []byte{}, 0o644))

	router := newOnboardingRouter(t, openOnboardingTestDB(t), store)
	recorder, body := getOnboarding(t, router)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, false, body["needed"])
}

func TestOnboardingAutoCreatedConfigNeedsOnboarding(t *testing.T) {
	store, dir, _ := newOnboardingEnv(t, onboardingBaseConfig, true)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.ledger"), []byte{}, 0o644))

	router := newOnboardingRouter(t, openOnboardingTestDB(t), store)
	recorder, body := getOnboarding(t, router)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, true, body["needed"])

	profiles, ok := body["profiles"].([]any)
	require.True(t, ok)
	require.Len(t, profiles, 2)

	first := profiles[0].(map[string]any)
	second := profiles[1].(map[string]any)
	assert.Equal(t, "germany-eu", first["id"])
	assert.Equal(t, "EUR", first["currency"])
	assert.Equal(t, "de-DE", first["locale"])
	assert.Equal(t, "india", second["id"])
	assert.Equal(t, "INR", second["currency"])
	assert.Equal(t, "en-IN", second["locale"])
}

func TestOnboardingConcurrentOnlyOneSucceeds(t *testing.T) {
	store, dir, _ := newOnboardingEnv(t, onboardingBaseConfig, true)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.ledger"), []byte{}, 0o644))

	router := newOnboardingRouter(t, openOnboardingTestDB(t), store)

	results := make([]int, 2)
	var wait sync.WaitGroup
	for index := range results {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			recorder := postOnboarding(t, router,
				`{"profile":"germany-eu","mode":"empty"}`, onboardingLoopback, nil)
			results[index] = recorder.Code
		}(index)
	}
	wait.Wait()

	assert.ElementsMatch(t, []int{http.StatusOK, http.StatusConflict}, results)
	assert.True(t, store.Initialized())
	assert.False(t, store.AutoCreated())
}

func TestOnboardingReadonlyIsRejected(t *testing.T) {
	yaml := onboardingBaseConfig + "readonly: true\n"
	store, dir, configPath := newOnboardingEnv(t, yaml, true)
	journalPath := filepath.Join(dir, "main.ledger")
	require.NoError(t, os.WriteFile(journalPath, []byte{}, 0o644))

	configBefore, err := os.ReadFile(configPath)
	require.NoError(t, err)
	journalBefore, err := os.ReadFile(journalPath)
	require.NoError(t, err)

	router := newOnboardingRouter(t, openOnboardingTestDB(t), store)
	recorder := postOnboarding(t, router,
		`{"profile":"germany-eu","mode":"empty"}`, onboardingLoopback, nil)

	require.Equal(t, http.StatusForbidden, recorder.Code)

	configAfter, err := os.ReadFile(configPath)
	require.NoError(t, err)
	journalAfter, err := os.ReadFile(journalPath)
	require.NoError(t, err)
	assert.Equal(t, configBefore, configAfter)
	assert.Equal(t, journalBefore, journalAfter)
	assert.False(t, store.Initialized())
}

func TestOnboardingRefusesNonEmptyJournal(t *testing.T) {
	store, dir, _ := newOnboardingEnv(t, onboardingBaseConfig, true)
	journalPath := filepath.Join(dir, "main.ledger")
	journal := []byte("2024/01/01 Opening balance\n    Assets:Checking:ING    100,00 EUR\n    Equity:Opening\n")
	require.NoError(t, os.WriteFile(journalPath, journal, 0o644))

	router := newOnboardingRouter(t, openOnboardingTestDB(t), store)
	recorder := postOnboarding(t, router,
		`{"profile":"india","mode":"demo"}`, onboardingLoopback, nil)

	require.Equal(t, http.StatusConflict, recorder.Code)

	after, err := os.ReadFile(journalPath)
	require.NoError(t, err)
	assert.Equal(t, journal, after, "journal must stay byte-identical")
	assert.False(t, store.Initialized())
}

func TestOnboardingRejectsNonLoopbackWithoutAuth(t *testing.T) {
	store, dir, configPath := newOnboardingEnv(t, onboardingBaseConfig, true)
	journalPath := filepath.Join(dir, "main.ledger")
	require.NoError(t, os.WriteFile(journalPath, []byte{}, 0o644))

	configBefore, err := os.ReadFile(configPath)
	require.NoError(t, err)

	router := newOnboardingRouter(t, openOnboardingTestDB(t), store)
	recorder := postOnboarding(t, router,
		`{"profile":"germany-eu","mode":"empty"}`, "203.0.113.7:54321", nil)

	require.Equal(t, http.StatusForbidden, recorder.Code)

	configAfter, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Equal(t, configBefore, configAfter)
	assert.False(t, store.Initialized())
}

func TestOnboardingEmptyCreatesGermanyConfig(t *testing.T) {
	store, dir, configPath := newOnboardingEnv(t, onboardingBaseConfig, true)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.ledger"), []byte{}, 0o644))

	router := newOnboardingRouter(t, openOnboardingTestDB(t), store)
	recorder := postOnboarding(t, router,
		`{"profile":"germany-eu","mode":"empty"}`, onboardingLoopback, nil)

	require.Equal(t, http.StatusOK, recorder.Code)

	body := map[string]any{}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	assert.Equal(t, true, body["success"])
	assert.Equal(t, "germany-eu", body["profile"])
	assert.Equal(t, "empty", body["mode"])

	content, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "regional_profile: germany-eu")
	assert.Contains(t, string(content), "default_currency: EUR")

	assert.True(t, store.Initialized())
	assert.False(t, store.AutoCreated())
	assert.Equal(t, "EUR", store.Get().DefaultCurrency)
}

func TestOnboardingDemoSeedsPostings(t *testing.T) {
	store, dir, _ := newOnboardingEnv(t, onboardingBaseConfig, true)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.ledger"), []byte{}, 0o644))

	db := openOnboardingTestDB(t)
	router := newOnboardingRouter(t, db, store)
	recorder := postOnboarding(t, router,
		`{"profile":"germany-eu","mode":"demo"}`, onboardingLoopback, nil)

	require.Equal(t, http.StatusOK, recorder.Code)

	var count int64
	require.NoError(t, db.Model(&posting.Posting{}).Count(&count).Error)

	if count == 0 {
		// Development checkouts embed a dummy ledger binary. Exercise the
		// generated demo journal through hledger when it is available, so the
		// test still proves the demo content produces postings.
		if _, err := exec.LookPath("hledger"); err != nil {
			t.Skip("neither ledger nor hledger is available")
		}
		require.NoError(t, store.Update(func(next *config.Config) error {
			next.LedgerCli = "hledger"
			return nil
		}))
		outcome := Sync(Deps{DB: db, Config: store}, SyncRequest{Journal: true})
		require.Equal(t, true, outcome["success"], outcome)
		require.NoError(t, db.Model(&posting.Posting{}).Count(&count).Error)
	}

	assert.Positive(t, count)
}

// TestOnboardingPostIsReachableWithoutSession proves a fresh install without a
// session can POST when CSRF has no origin (CLI/tests) or a same-origin origin
// (SPA). Cross-origin requests must still be rejected.
func TestOnboardingPostIsReachableWithoutSession(t *testing.T) {
	t.Run("same origin", func(t *testing.T) {
		store, dir, _ := newOnboardingEnv(t, onboardingBaseConfig, true)
		require.NoError(t, os.WriteFile(filepath.Join(dir, "main.ledger"), []byte{}, 0o644))

		router := newOnboardingRouter(t, openOnboardingTestDB(t), store)
		recorder := postOnboarding(t, router,
			`{"profile":"germany-eu","mode":"empty"}`, onboardingLoopback,
			map[string]string{"Origin": "http://example.com"})

		require.Equal(t, http.StatusOK, recorder.Code)
	})

	t.Run("cross origin stays blocked", func(t *testing.T) {
		store, dir, _ := newOnboardingEnv(t, onboardingBaseConfig, true)
		require.NoError(t, os.WriteFile(filepath.Join(dir, "main.ledger"), []byte{}, 0o644))

		router := newOnboardingRouter(t, openOnboardingTestDB(t), store)
		recorder := postOnboarding(t, router,
			`{"profile":"germany-eu","mode":"empty"}`, onboardingLoopback,
			map[string]string{"Origin": "http://evil.example"})

		require.Equal(t, http.StatusForbidden, recorder.Code)
		assert.False(t, store.Initialized())
	})
}
