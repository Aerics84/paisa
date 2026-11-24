package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/model/cache"
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/ananthakumaran/paisa/internal/model/price"
	"github.com/ananthakumaran/paisa/internal/pricesync"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openSyncTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	return openSyncTestDBWithConfig(t, `
journal_path: main.ledger
db_path: paisa.db
default_currency: EUR
time_zone: UTC
`)
}

func openSyncTestDBWithConfig(t *testing.T, content string) *gorm.DB {
	t.Helper()

	require.NoError(t, config.LoadConfig([]byte(content), ""))

	name := strings.NewReplacer("/", "_", "\\", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&posting.Posting{}, &price.Price{}, &cache.Cache{}))

	return db
}

// testPriceManager returns a manager whose sync function never touches the
// network. Tests that need to control the sync function construct their own
// manager inline.
func testPriceManager(t *testing.T, db *gorm.DB) *pricesync.Manager {
	return pricesync.New(db, config.GetConfig, pricesync.NewMemoryStateStore(),
		func(pricesync.SyncOptions) pricesync.Outcome {
			return pricesync.Outcome{Success: true}
		}, pricesync.NewRunner(), time.Now)
}

func TestNewPriceManagerNonNil(t *testing.T) {
	db := openSyncTestDB(t)
	manager := NewPriceManager(db, pricesync.NewMemoryStateStore())
	require.NotNil(t, manager)
}

func TestSyncStatusEndpoint(t *testing.T) {
	db := openSyncTestDB(t)
	manager := testPriceManager(t, db)
	router := BuildWithManager(Deps{DB: db, Config: config.DefaultStore()}, false, manager)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/sync/status", nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var status pricesync.Status
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &status))
	assert.False(t, status.Running)
	assert.False(t, status.Managed) // no commodities in the test config
}

func TestBackgroundSyncReturns202AndStartsOneJob(t *testing.T) {
	db := openSyncTestDB(t)
	manager := testPriceManager(t, db)
	router := BuildWithManager(Deps{DB: db, Config: config.DefaultStore()}, false, manager)

	body := `{"prices":true,"background":true}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/sync", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusAccepted, w.Code)
	var response struct {
		Success bool             `json:"success"`
		Started bool             `json:"started"`
		Status  pricesync.Status `json:"status"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.True(t, response.Success)
	assert.True(t, response.Started)
}

func TestBackgroundSyncIgnoresJournalAndPortfolios(t *testing.T) {
	db := openSyncTestDB(t)
	manager := testPriceManager(t, db)
	router := BuildWithManager(Deps{DB: db, Config: config.DefaultStore()}, false, manager)

	for _, body := range []string{
		`{"prices":true,"journal":true,"background":true}`,
		`{"prices":true,"portfolios":true,"background":true}`,
	} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/sync", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code, "body %q", body)
	}
}

func TestBackgroundSyncRequiresPrices(t *testing.T) {
	db := openSyncTestDB(t)

	var calls int32
	syncFn := func(pricesync.SyncOptions) pricesync.Outcome {
		atomic.AddInt32(&calls, 1)
		return pricesync.Outcome{Success: true}
	}
	manager := pricesync.New(db, config.GetConfig, pricesync.NewMemoryStateStore(), syncFn, pricesync.NewRunner(), time.Now)
	router := BuildWithManager(Deps{DB: db, Config: config.DefaultStore()}, false, manager)

	body := `{"background":true}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/sync", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, int32(0), atomic.LoadInt32(&calls))
}

func TestReadonlyBackgroundSyncIsNoop(t *testing.T) {
	db := openSyncTestDBWithConfig(t, `
journal_path: main.ledger
db_path: paisa.db
default_currency: EUR
time_zone: UTC
readonly: true
`)

	var calls int32
	syncFn := func(pricesync.SyncOptions) pricesync.Outcome {
		atomic.AddInt32(&calls, 1)
		return pricesync.Outcome{Success: true}
	}
	manager := pricesync.New(db, config.GetConfig, pricesync.NewMemoryStateStore(), syncFn, pricesync.NewRunner(), time.Now)
	router := BuildWithManager(Deps{DB: db, Config: config.DefaultStore()}, false, manager)

	body := `{"prices":true,"background":true}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/sync", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int32(0), atomic.LoadInt32(&calls))
}

func TestSyncStatusIncludesJobID(t *testing.T) {
	db := openSyncTestDB(t)

	start := make(chan struct{})
	release := make(chan struct{})
	syncFn := func(pricesync.SyncOptions) pricesync.Outcome {
		close(start)
		<-release
		return pricesync.Outcome{Success: true}
	}
	manager := pricesync.New(db, config.GetConfig, pricesync.NewMemoryStateStore(), syncFn, pricesync.NewRunner(), time.Now)
	router := BuildWithManager(Deps{DB: db, Config: config.DefaultStore()}, false, manager)

	body := `{"prices":true,"background":true}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/sync", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusAccepted, w.Code)
	<-start // sync function is now blocked

	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/api/sync/status", nil)
	router.ServeHTTP(w2, req2)
	require.Equal(t, http.StatusOK, w2.Code)

	var status pricesync.Status
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &status))
	assert.True(t, status.Running)
	assert.Greater(t, status.JobID, uint64(0))

	close(release)
}

func TestConcurrentBackgroundRequestsStartOneJob(t *testing.T) {
	db := openSyncTestDB(t)

	var calls int32
	release := make(chan struct{})
	syncFn := func(pricesync.SyncOptions) pricesync.Outcome {
		atomic.AddInt32(&calls, 1)
		<-release
		return pricesync.Outcome{Success: true}
	}
	manager := pricesync.New(db, config.GetConfig, pricesync.NewMemoryStateStore(), syncFn, pricesync.NewRunner(), time.Now)
	router := BuildWithManager(Deps{DB: db, Config: config.DefaultStore()}, false, manager)

	body := `{"prices":true,"background":true}`

	const requests = 10
	codes := make([]int, requests)
	started := make([]bool, requests)

	var wg sync.WaitGroup
	for i := 0; i < requests; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/sync", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			codes[i] = w.Code
			var response struct {
				Started bool `json:"started"`
			}
			_ = json.Unmarshal(w.Body.Bytes(), &response)
			started[i] = response.Started
		}(i)
	}
	wg.Wait()

	startedCount := 0
	for _, s := range started {
		if s {
			startedCount++
		}
	}
	assert.Equal(t, 1, startedCount)

	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&calls) == 1
	}, time.Second, 10*time.Millisecond)
	assert.Equal(t, int32(1), atomic.LoadInt32(&calls))

	for _, code := range codes {
		assert.Equal(t, http.StatusAccepted, code)
	}

	close(release)
}

func TestSyncResponseKeepsLegacySuccessShape(t *testing.T) {
	assert.Equal(t, gin.H{"success": true}, syncResponse(pricesync.Outcome{Success: true}))
}

func TestSyncResponseIncludesPartialDetails(t *testing.T) {
	got := syncResponse(pricesync.Outcome{
		Success:           false,
		Partial:           true,
		FailedCommodities: []string{"NFLX"},
		Error:             "boom",
		Warning:           "CI",
	})
	assert.Equal(t, gin.H{
		"success":            false,
		"message":            "boom",
		"partial":            true,
		"failed_commodities": []string{"NFLX"},
		"warning":            "CI",
	}, got)
}
