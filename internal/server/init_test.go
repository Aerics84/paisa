package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func postInit(t *testing.T, router *gin.Engine, body string, remoteAddr string) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(http.MethodPost, "/api/init", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.RemoteAddr = remoteAddr
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	return recorder
}

func getInit(t *testing.T, router *gin.Engine, remoteAddr string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()

	request := httptest.NewRequest(http.MethodGet, "/api/init", nil)
	request.RemoteAddr = remoteAddr
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	body := map[string]any{}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))

	return recorder, body
}

// stubInitSyncSeedsPostings makes the required journal phase succeed and
// insert one posting, which mirrors what a real journal sync does.
func stubInitSyncSeedsPostings(t *testing.T) {
	t.Helper()

	stubOnboardingSyncByRequest(t, func(deps Deps, request SyncRequest) gin.H {
		if request.Journal {
			if err := deps.DB.Create(&posting.Posting{
				Date:      time.Now(),
				Payee:     "Seeded by test",
				Account:   "Assets:Checking:ING",
				Commodity: "EUR",
				Quantity:  decimal.NewFromInt(1),
				Amount:    decimal.NewFromInt(1),
			}).Error; err != nil {
				t.Errorf("could not seed demo posting: %v", err)
			}
		}
		return gin.H{"success": true}
	})
}

func TestInitRefusesNonEmptyJournalWithoutTouchingFiles(t *testing.T) {
	store, dir, configPath := newOnboardingEnv(t, onboardingBaseConfig, false)
	journalPath := filepath.Join(dir, "main.ledger")
	journal := []byte("2024/01/01 Opening balance\n    Assets:Checking:ING    100,00 EUR\n    Equity:Opening\n")
	require.NoError(t, os.WriteFile(journalPath, journal, 0o644))

	configBefore, err := os.ReadFile(configPath)
	require.NoError(t, err)

	router := newOnboardingRouter(t, openOnboardingTestDB(t), store)
	recorder := postInit(t, router, `{"regional_profile":"germany-eu"}`, onboardingLoopback)

	require.Equal(t, http.StatusConflict, recorder.Code)

	journalAfter, err := os.ReadFile(journalPath)
	require.NoError(t, err)
	assert.Equal(t, journal, journalAfter, "journal must stay byte-identical")

	configAfter, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Equal(t, configBefore, configAfter, "config must stay byte-identical")

	_, err = os.Stat(journalPath + ".onboarding-backup")
	assert.ErrorIs(t, err, os.ErrNotExist)
	_, err = os.Stat(configPath + ".onboarding-backup")
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestInitWithoutAuthRequiresLoopback(t *testing.T) {
	store, dir, configPath := newOnboardingEnv(t, onboardingBaseConfig, false)
	journalPath := filepath.Join(dir, "main.ledger")
	require.NoError(t, os.WriteFile(journalPath, []byte{}, 0o644))

	configBefore, err := os.ReadFile(configPath)
	require.NoError(t, err)

	router := newOnboardingRouter(t, openOnboardingTestDB(t), store)
	recorder := postInit(t, router, `{"regional_profile":"germany-eu"}`, "203.0.113.7:54321")

	require.Equal(t, http.StatusForbidden, recorder.Code)

	configAfter, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Equal(t, configBefore, configAfter)
}

func TestInitRemoteOptInAllowsDemoSetup(t *testing.T) {
	t.Setenv("PAISA_ALLOW_REMOTE_ONBOARDING", "1")

	store, dir, _ := newOnboardingEnv(t, onboardingBaseConfig, false)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.ledger"), []byte{}, 0o644))
	stubInitSyncSeedsPostings(t)

	router := newOnboardingRouter(t, openOnboardingTestDB(t), store)
	recorder := postInit(t, router, `{"regional_profile":"germany-eu"}`, "203.0.113.7:54321")

	require.Equal(t, http.StatusOK, recorder.Code)

	body := map[string]any{}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	assert.Equal(t, true, body["success"])
	assert.Equal(t, "germany-eu", body["profile"])
	assert.Equal(t, "demo", body["mode"])
}

func TestInitStatusReportsWhetherDemoSetupIsPossible(t *testing.T) {
	store, dir, _ := newOnboardingEnv(t, onboardingBaseConfig, false)
	journalPath := filepath.Join(dir, "main.ledger")
	require.NoError(t, os.WriteFile(journalPath, []byte{}, 0o644))

	router := newOnboardingRouter(t, openOnboardingTestDB(t), store)

	recorder, body := getInit(t, router, onboardingLoopback)
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, true, body["can_init"])

	require.NoError(t, os.WriteFile(journalPath, []byte("2024/01/01 Opening\n    Assets:Checking:ING  1 EUR\n    Equity:Opening\n"), 0o644))
	_, body = getInit(t, router, onboardingLoopback)
	assert.Equal(t, false, body["can_init"])
	assert.Contains(t, body["reason"], "journal already contains data")

	recorder, body = getInit(t, router, "203.0.113.7:54321")
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, false, body["can_init"])
	assert.Contains(t, body["reason"], "localhost")
}

func TestInitStatusRejectsReadonly(t *testing.T) {
	store, dir, _ := newOnboardingEnv(t, onboardingBaseConfig+"readonly: true\n", false)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.ledger"), []byte{}, 0o644))

	router := newOnboardingRouter(t, openOnboardingTestDB(t), store)
	_, body := getInit(t, router, onboardingLoopback)

	assert.Equal(t, false, body["can_init"])
	assert.Contains(t, body["reason"], "read-only")
}

func TestInitFailedSyncRestoresFiles(t *testing.T) {
	store, dir, configPath := newOnboardingEnv(t, onboardingBaseConfig, false)
	journalPath := filepath.Join(dir, "main.ledger")
	require.NoError(t, os.WriteFile(journalPath, []byte{}, 0o644))

	configBefore, err := os.ReadFile(configPath)
	require.NoError(t, err)
	journalBefore, err := os.ReadFile(journalPath)
	require.NoError(t, err)

	stubOnboardingSyncByRequest(t, func(Deps, SyncRequest) gin.H {
		return gin.H{"success": false, "message": "ledger unavailable"}
	})

	router := newOnboardingRouter(t, openOnboardingTestDB(t), store)
	recorder := postInit(t, router, `{"regional_profile":"germany-eu"}`, onboardingLoopback)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)

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
	assert.ErrorIs(t, err, os.ErrNotExist)
	_, err = os.Stat(configPath + ".onboarding-backup")
	assert.ErrorIs(t, err, os.ErrNotExist)
	_, err = os.Stat(journalPath + ".onboarding-backup")
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestInitWithoutPostingsRollsBack(t *testing.T) {
	store, dir, configPath := newOnboardingEnv(t, onboardingBaseConfig, false)
	journalPath := filepath.Join(dir, "main.ledger")
	require.NoError(t, os.WriteFile(journalPath, []byte{}, 0o644))

	configBefore, err := os.ReadFile(configPath)
	require.NoError(t, err)

	// The journal phase succeeds but does not seed any posting; that must be
	// treated as a failed demo setup.
	stubOnboardingSync(t, gin.H{"success": true})

	router := newOnboardingRouter(t, openOnboardingTestDB(t), store)
	recorder := postInit(t, router, `{"regional_profile":"germany-eu"}`, onboardingLoopback)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)

	configAfter, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Equal(t, configBefore, configAfter)

	journalAfter, err := os.ReadFile(journalPath)
	require.NoError(t, err)
	assert.Empty(t, journalAfter)
}

func TestInitConcurrentAttemptsOnlyOneSucceeds(t *testing.T) {
	store, dir, _ := newOnboardingEnv(t, onboardingBaseConfig, false)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.ledger"), []byte{}, 0o644))
	stubInitSyncSeedsPostings(t)

	router := newOnboardingRouter(t, openOnboardingTestDB(t), store)

	results := make([]int, 2)
	var wait sync.WaitGroup
	for index := range results {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			recorder := postInit(t, router, `{"regional_profile":"germany-eu"}`, onboardingLoopback)
			results[index] = recorder.Code
		}(index)
	}
	wait.Wait()

	assert.ElementsMatch(t, []int{http.StatusOK, http.StatusConflict}, results)
}
