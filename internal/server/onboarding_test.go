package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	"github.com/shopspring/decimal"
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

	return getOnboardingFrom(t, router, onboardingLoopback)
}

func getOnboardingFrom(t *testing.T, router *gin.Engine, remoteAddr string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()

	request := httptest.NewRequest(http.MethodGet, "/api/onboarding", nil)
	request.RemoteAddr = remoteAddr
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

// stubOnboardingSync replaces the sync step so tests that are not about ledger
// parsing do not depend on an external ledger binary.
func stubOnboardingSync(t *testing.T, result gin.H) {
	t.Helper()

	previous := onboardingSync
	onboardingSync = func(Deps, SyncRequest) gin.H { return result }
	t.Cleanup(func() { onboardingSync = previous })
}

// stubOnboardingSyncByRequest lets a test distinguish the required journal
// phase from the best-effort price/portfolio enrichment phase.
func stubOnboardingSyncByRequest(t *testing.T, result func(Deps, SyncRequest) gin.H) {
	t.Helper()

	previous := onboardingSync
	onboardingSync = result
	t.Cleanup(func() { onboardingSync = previous })
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
	assert.Equal(t, true, body["can_setup"])

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

func TestOnboardingRemoteClientCannotSetup(t *testing.T) {
	store, dir, _ := newOnboardingEnv(t, onboardingBaseConfig, true)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.ledger"), []byte{}, 0o644))

	router := newOnboardingRouter(t, openOnboardingTestDB(t), store)
	recorder, body := getOnboardingFrom(t, router, "203.0.113.7:54321")

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, true, body["needed"])
	assert.Equal(t, false, body["can_setup"], "remote first-run must not advertise a setup that POST rejects")

	// The POST guard is unchanged: a remote client still gets 403 unless the
	// operator opts in explicitly.
	post := postOnboarding(t, router,
		`{"profile":"germany-eu","mode":"empty"}`, "203.0.113.7:54321", nil)
	require.Equal(t, http.StatusForbidden, post.Code)
	assert.False(t, store.Initialized())
}

func TestOnboardingRemoteOptInAllowsEmptySetup(t *testing.T) {
	t.Setenv("PAISA_ALLOW_REMOTE_ONBOARDING", "1")

	store, dir, configPath := newOnboardingEnv(t, onboardingBaseConfig, true)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.ledger"), []byte{}, 0o644))

	router := newOnboardingRouter(t, openOnboardingTestDB(t), store)
	recorder, body := getOnboardingFrom(t, router, "203.0.113.7:54321")

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, true, body["needed"])
	assert.Equal(t, true, body["can_setup"])

	// Empty setup must not need a ledger parser, so this runs the real sync
	// seam rather than a stub.
	post := postOnboarding(t, router,
		`{"profile":"germany-eu","mode":"empty"}`, "203.0.113.7:54321", nil)
	require.Equal(t, http.StatusOK, post.Code)

	result := map[string]any{}
	require.NoError(t, json.Unmarshal(post.Body.Bytes(), &result))
	assert.Equal(t, true, result["success"])
	assert.True(t, store.Initialized())
	assert.False(t, store.AutoCreated())

	content, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "regional_profile: germany-eu")
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

	// Development checkouts embed a dummy ledger binary. When no real ledger is
	// on PATH, run the generated demo journal through hledger instead. The test
	// fails loudly when neither parser is available rather than silently
	// skipping.
	if _, err := exec.LookPath("ledger"); err != nil {
		if _, err := exec.LookPath("hledger"); err != nil {
			t.Fatal("onboarding demo test requires ledger or hledger on PATH")
		}

		previous := onboardingSync
		onboardingSync = func(deps Deps, request SyncRequest) gin.H {
			require.NoError(t, deps.Config.Update(func(next *config.Config) error {
				next.LedgerCli = "hledger"
				return nil
			}))
			return Sync(deps, request)
		}
		t.Cleanup(func() { onboardingSync = previous })
	}

	db := openOnboardingTestDB(t)
	router := newOnboardingRouter(t, db, store)
	recorder := postOnboarding(t, router,
		`{"profile":"germany-eu","mode":"demo"}`, onboardingLoopback, nil)

	require.Equal(t, http.StatusOK, recorder.Code)

	var count int64
	require.NoError(t, db.Model(&posting.Posting{}).Count(&count).Error)
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

func TestOnboardingConflictWhenAlreadyInitialized(t *testing.T) {
	store, dir, configPath := newOnboardingEnv(t, onboardingBaseConfig, true)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.ledger"), []byte{}, 0o644))
	// An auto-created installation that already loaded a configuration must not
	// start onboarding again.
	require.NoError(t, store.Load([]byte(onboardingBaseConfig), configPath))
	require.True(t, store.Initialized())
	require.True(t, store.AutoCreated())

	router := newOnboardingRouter(t, openOnboardingTestDB(t), store)
	recorder := postOnboarding(t, router,
		`{"profile":"germany-eu","mode":"empty"}`, onboardingLoopback, nil)

	require.Equal(t, http.StatusConflict, recorder.Code)
}

func TestOnboardingConflictWhenNotAutoCreated(t *testing.T) {
	// A user-managed configuration is neither auto-created nor initialized in
	// this fresh store; onboarding must still be refused.
	store := config.NewStore()
	router := newOnboardingRouter(t, openOnboardingTestDB(t), store)

	recorder := postOnboarding(t, router,
		`{"profile":"germany-eu","mode":"empty"}`, onboardingLoopback, nil)

	require.Equal(t, http.StatusConflict, recorder.Code)
	assert.False(t, store.Initialized())
}

func TestOnboardingRejectsSpoofedForwardedFor(t *testing.T) {
	store, dir, configPath := newOnboardingEnv(t, onboardingBaseConfig, true)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.ledger"), []byte{}, 0o644))

	configBefore, err := os.ReadFile(configPath)
	require.NoError(t, err)

	router := newOnboardingRouter(t, openOnboardingTestDB(t), store)
	recorder := postOnboarding(t, router,
		`{"profile":"germany-eu","mode":"empty"}`, "203.0.113.7:54321",
		map[string]string{"X-Forwarded-For": "127.0.0.1"})

	require.Equal(t, http.StatusForbidden, recorder.Code)

	configAfter, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Equal(t, configBefore, configAfter)
	assert.False(t, store.Initialized())
}

func TestOnboardingFailedSyncRestoresFilesAndAllowsRetry(t *testing.T) {
	store, dir, configPath := newOnboardingEnv(t, onboardingBaseConfig, true)
	journalPath := filepath.Join(dir, "main.ledger")
	require.NoError(t, os.WriteFile(journalPath, []byte{}, 0o640))

	configBefore, err := os.ReadFile(configPath)
	require.NoError(t, err)
	journalBefore, err := os.ReadFile(journalPath)
	require.NoError(t, err)

	calls := []SyncRequest{}
	stubOnboardingSyncByRequest(t, func(_ Deps, request SyncRequest) gin.H {
		calls = append(calls, request)
		return gin.H{"success": false, "message": "ledger unavailable"}
	})

	router := newOnboardingRouter(t, openOnboardingTestDB(t), store)
	// Germany demo generation is fully offline, so only the injected journal
	// sync failure is exercised here (no India NAV calls).
	recorder := postOnboarding(t, router,
		`{"profile":"germany-eu","mode":"demo"}`, onboardingLoopback, nil)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	require.Len(t, calls, 1, "enrichment must not run after a required journal failure")
	assert.True(t, calls[0].Journal)
	assert.False(t, calls[0].Prices)
	body := map[string]any{}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	assert.Equal(t, "ledger unavailable", body["error"])

	configAfter, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Equal(t, configBefore, configAfter, "config must be restored")
	journalAfter, err := os.ReadFile(journalPath)
	require.NoError(t, err)
	assert.Equal(t, journalBefore, journalAfter, "journal must be restored")

	_, err = os.Stat(filepath.Join(dir, "Schedule AL.paisa"))
	assert.ErrorIs(t, err, os.ErrNotExist, "generated sheet must be removed")
	_, err = os.Stat(configPath + ".onboarding-backup")
	assert.ErrorIs(t, err, os.ErrNotExist, "backup must be cleaned up after restore")
	_, err = os.Stat(journalPath + ".onboarding-backup")
	assert.ErrorIs(t, err, os.ErrNotExist, "backup must be cleaned up after restore")

	assert.True(t, store.AutoCreated())
	assert.False(t, store.Initialized(), "a failed attempt must remain retryable")

	_, status := getOnboarding(t, router)
	assert.Equal(t, true, status["needed"])

	// A retry with a working sync must succeed on the restored files. Empty
	// mode has no postings requirement, so the stub is enough.
	onboardingSync = func(Deps, SyncRequest) gin.H { return gin.H{"success": true} }
	recorder = postOnboarding(t, router,
		`{"profile":"germany-eu","mode":"empty"}`, onboardingLoopback, nil)
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.True(t, store.Initialized())
	assert.False(t, store.AutoCreated())
}

func TestOnboardingDemoWithoutPostingsFails(t *testing.T) {
	store, dir, configPath := newOnboardingEnv(t, onboardingBaseConfig, true)
	journalPath := filepath.Join(dir, "main.ledger")
	require.NoError(t, os.WriteFile(journalPath, []byte{}, 0o644))

	configBefore, err := os.ReadFile(configPath)
	require.NoError(t, err)

	stubOnboardingSync(t, gin.H{"success": true})

	router := newOnboardingRouter(t, openOnboardingTestDB(t), store)
	recorder := postOnboarding(t, router,
		`{"profile":"germany-eu","mode":"demo"}`, onboardingLoopback, nil)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)

	configAfter, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Equal(t, configBefore, configAfter)
	assert.False(t, store.Initialized())
	assert.True(t, store.AutoCreated())
}

func TestOnboardingEnrichmentFailureKeepsEmptySuccess(t *testing.T) {
	store, dir, configPath := newOnboardingEnv(t, onboardingBaseConfig, true)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.ledger"), []byte{}, 0o644))

	stubOnboardingSyncByRequest(t, func(_ Deps, request SyncRequest) gin.H {
		if request.Prices {
			return gin.H{"success": false, "message": "price sync failed: NIFTY"}
		}
		return gin.H{"success": true}
	})

	router := newOnboardingRouter(t, openOnboardingTestDB(t), store)
	recorder := postOnboarding(t, router,
		`{"profile":"germany-eu","mode":"empty"}`, onboardingLoopback, nil)

	require.Equal(t, http.StatusOK, recorder.Code)

	body := map[string]any{}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	assert.Equal(t, true, body["success"])
	assert.Equal(t, "price sync failed: NIFTY", body["warning"])

	content, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "regional_profile: germany-eu")
	assert.True(t, store.Initialized(), "enrichment failure must not undo onboarding")
	assert.False(t, store.AutoCreated())
}

func TestOnboardingDemoEnrichmentFailureKeepsSuccess(t *testing.T) {
	store, dir, _ := newOnboardingEnv(t, onboardingBaseConfig, true)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.ledger"), []byte{}, 0o644))

	db := openOnboardingTestDB(t)
	stubOnboardingSyncByRequest(t, func(deps Deps, request SyncRequest) gin.H {
		if request.Prices {
			return gin.H{"success": false, "message": "price sync failed: NIFTY"}
		}
		// The required phase is responsible for seeding postings; insert one so
		// the demo postings check passes without a real ledger binary.
		require.NoError(t, deps.DB.Create(&posting.Posting{
			Date:      time.Now(),
			Payee:     "Seeded by test",
			Account:   "Assets:Checking:SBI",
			Commodity: "INR",
			Quantity:  decimal.NewFromInt(1),
			Amount:    decimal.NewFromInt(1),
		}).Error)
		return gin.H{"success": true}
	})

	router := newOnboardingRouter(t, db, store)
	recorder := postOnboarding(t, router,
		`{"profile":"germany-eu","mode":"demo"}`, onboardingLoopback, nil)

	require.Equal(t, http.StatusOK, recorder.Code)

	body := map[string]any{}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	assert.Equal(t, true, body["success"])
	assert.Equal(t, "price sync failed: NIFTY", body["warning"])
	assert.True(t, store.Initialized())
	assert.False(t, store.AutoCreated())
}

func TestRestoreOnboardingBackupsPreservesContentAndMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.ledger")
	require.NoError(t, os.WriteFile(path, []byte("original"), 0o640))

	backups, err := backupOnboardingFiles(path)
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(path, []byte("generated"), 0o600))
	restoreOnboardingBackups(backups)

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "original", string(content))

	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o640), info.Mode().Perm())
	}

	_, err = os.Stat(path + ".onboarding-backup")
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestRemoveOnboardingBackupsAfterSuccess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "paisa.yaml")
	require.NoError(t, os.WriteFile(path, []byte("original"), 0o644))

	backups, err := backupOnboardingFiles(path)
	require.NoError(t, err)
	require.FileExists(t, path+".onboarding-backup")

	removeOnboardingBackups(backups)
	_, err = os.Stat(path + ".onboarding-backup")
	assert.ErrorIs(t, err, os.ErrNotExist)
}
