package server

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ananthakumaran/paisa/internal/auth"
	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/utils"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testRouter(t *testing.T, yaml string) *gin.Engine {
	t.Helper()

	path := filepath.Join(t.TempDir(), "paisa.yaml")
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0o600))

	router, _ := testRouterWithPath(t, yaml, path)
	return router
}

// testRouterWithPath builds a router backed by an isolated config store bound
// to path. The file is not written here so tests can deliberately bind the
// store to an unwritable location.
func testRouterWithPath(t *testing.T, yaml string, path string) (*gin.Engine, *config.Store) {
	t.Helper()

	store := config.NewStore()
	require.NoError(t, store.Load([]byte(yaml), path))

	router := Build(Deps{DB: nil, Config: store, Auth: auth.NewStore(time.Hour)}, false)
	return router, store
}

func legacyAccountConfig(password string, readonly bool) string {
	readonlyLine := ""
	if readonly {
		readonlyLine = "readonly: true\n"
	}

	return readonlyLine + fmt.Sprintf(`journal_path: main.ledger
db_path: paisa.db
user_accounts:
  - username: alice
    password: "sha256:%s"
`, utils.Sha256(utils.Sha256(password)))
}

func argon2AccountConfig(t *testing.T, password string) string {
	t.Helper()

	hash, err := utils.HashPassword(password)
	require.NoError(t, err)

	return fmt.Sprintf(`journal_path: main.ledger
db_path: paisa.db
user_accounts:
  - username: alice
    password: %q
`, hash)
}

const noAccountConfig = `journal_path: main.ledger
db_path: paisa.db
`

func postJSON(t *testing.T, router *gin.Engine, path string, body string, mutate func(*http.Request)) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if mutate != nil {
		mutate(req)
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func getWithCookie(router *gin.Engine, path string, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func doLogin(t *testing.T, router *gin.Engine, username string, password string) *httptest.ResponseRecorder {
	t.Helper()

	return postJSON(t, router, "/api/auth/login",
		fmt.Sprintf(`{"username":%q,"password":%q}`, username, password), nil)
}

func requireSessionCookie(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	require.Equal(t, http.StatusOK, rec.Code)
	cookies := rec.Result().Cookies()
	require.NotEmpty(t, cookies)
	return cookies[0]
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()

	body := map[string]any{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return body
}

func TestLoginAcceptsLegacyHashAndIssuesCookie(t *testing.T) {
	router := testRouter(t, legacyAccountConfig("secret", false))

	rec := doLogin(t, router, "alice", "secret")
	require.Equal(t, http.StatusOK, rec.Code)

	cookie := requireSessionCookie(t, rec)
	assert.Equal(t, "paisa_session", cookie.Name)
	assert.True(t, cookie.HttpOnly)
	assert.Equal(t, http.SameSiteStrictMode, cookie.SameSite)
	assert.Equal(t, "/", cookie.Path)
	assert.Positive(t, cookie.MaxAge)

	body := decodeBody(t, rec)
	assert.Equal(t, true, body["success"])
	assert.Equal(t, "alice", body["username"])
}

func TestLoginRejectsWrongPassword(t *testing.T) {
	router := testRouter(t, legacyAccountConfig("secret", false))

	rec := doLogin(t, router, "alice", "wrong")
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Empty(t, rec.Result().Cookies())
}

func TestLoginRejectsUnknownUser(t *testing.T) {
	router := testRouter(t, legacyAccountConfig("secret", false))

	rec := doLogin(t, router, "bob", "secret")
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Empty(t, rec.Result().Cookies())
}

func TestLoginAcceptsArgon2Hash(t *testing.T) {
	router := testRouter(t, argon2AccountConfig(t, "secret"))

	rec := doLogin(t, router, "alice", "secret")
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestStatusReportsDisabledAuthWithoutUsers(t *testing.T) {
	router := testRouter(t, noAccountConfig)

	rec := getWithCookie(router, "/api/auth/status", nil)
	require.Equal(t, http.StatusOK, rec.Code)

	body := decodeBody(t, rec)
	assert.Equal(t, false, body["auth_enabled"])
	assert.Equal(t, false, body["authenticated"])
	assert.Equal(t, "", body["username"])
}

func TestLoginRateLimitsRepeatedFailures(t *testing.T) {
	router := testRouter(t, legacyAccountConfig("secret", false))

	for attempt := 0; attempt < 4; attempt++ {
		assert.Equal(t, http.StatusUnauthorized, doLogin(t, router, "alice", "wrong").Code,
			"failed attempt %d should be rejected with 401", attempt+1)
	}

	rec := doLogin(t, router, "alice", "wrong")
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.Contains(t, strings.ToLower(string(rec.Body.Bytes())), "too many login attempts")
}

func TestSuccessfulLoginsAreNotRateLimited(t *testing.T) {
	router := testRouter(t, argon2AccountConfig(t, "secret"))

	for attempt := 0; attempt < 5; attempt++ {
		rec := doLogin(t, router, "alice", "secret")
		require.Equal(t, http.StatusOK, rec.Code, "login %d should succeed", attempt+1)
	}
}

func TestPublicAuthPathsBypassGeneralRateLimit(t *testing.T) {
	router := testRouter(t, argon2AccountConfig(t, "secret"))

	// Four invalid token attempts consume the general limiter budget; the
	// fifth request to a protected endpoint is rejected.
	var last *httptest.ResponseRecorder
	for attempt := 0; attempt < 5; attempt++ {
		last = getWithCookie(router, "/api/version", nil)
	}
	require.Equal(t, http.StatusTooManyRequests, last.Code)

	// Public auth paths must still work even while the general limiter is
	// exhausted. Bad credentials use the login-specific limiter, not the
	// middleware budget.
	bad := doLogin(t, router, "alice", "wrong")
	assert.Equal(t, http.StatusUnauthorized, bad.Code)

	good := doLogin(t, router, "alice", "secret")
	require.Equal(t, http.StatusOK, good.Code)

	status := getWithCookie(router, "/api/auth/status", nil)
	assert.Equal(t, http.StatusOK, status.Code)
}

func TestStatusReportsEnabledAuthAndSession(t *testing.T) {
	router := testRouter(t, legacyAccountConfig("secret", false))

	before := decodeBody(t, getWithCookie(router, "/api/auth/status", nil))
	assert.Equal(t, true, before["auth_enabled"])
	assert.Equal(t, false, before["authenticated"])
	assert.Equal(t, "", before["username"])

	cookie := requireSessionCookie(t, doLogin(t, router, "alice", "secret"))

	after := decodeBody(t, getWithCookie(router, "/api/auth/status", cookie))
	assert.Equal(t, true, after["auth_enabled"])
	assert.Equal(t, true, after["authenticated"])
	assert.Equal(t, "alice", after["username"])
}

func TestLogoutClearsCookie(t *testing.T) {
	router := testRouter(t, legacyAccountConfig("secret", false))
	cookie := requireSessionCookie(t, doLogin(t, router, "alice", "secret"))

	rec := postJSON(t, router, "/api/auth/logout", "", func(req *http.Request) {
		req.AddCookie(cookie)
	})
	require.Equal(t, http.StatusOK, rec.Code)

	cookies := rec.Result().Cookies()
	require.NotEmpty(t, cookies)
	assert.Equal(t, "paisa_session", cookies[0].Name)
	assert.Equal(t, -1, cookies[0].MaxAge)
	assert.Empty(t, cookies[0].Value)

	body := decodeBody(t, rec)
	assert.Equal(t, true, body["success"])
}

func TestSessionIsInvalidAfterLogout(t *testing.T) {
	router := testRouter(t, legacyAccountConfig("secret", false))
	cookie := requireSessionCookie(t, doLogin(t, router, "alice", "secret"))

	require.Equal(t, http.StatusOK, getWithCookie(router, "/api/version", cookie).Code)

	rec := postJSON(t, router, "/api/auth/logout", "", func(req *http.Request) {
		req.AddCookie(cookie)
	})
	require.Equal(t, http.StatusOK, rec.Code)

	assert.Equal(t, http.StatusUnauthorized, getWithCookie(router, "/api/version", cookie).Code)
}

func TestSessionCookieAuthenticatesProtectedEndpoint(t *testing.T) {
	router := testRouter(t, legacyAccountConfig("secret", false))
	cookie := requireSessionCookie(t, doLogin(t, router, "alice", "secret"))

	assert.Equal(t, http.StatusOK, getWithCookie(router, "/api/version", cookie).Code)
}

func TestForeignOriginIsRejectedForWrites(t *testing.T) {
	router := testRouter(t, legacyAccountConfig("secret", false))

	rec := postJSON(t, router, "/api/auth/login", `{"username":"alice","password":"secret"}`, func(req *http.Request) {
		req.Header.Set("Origin", "https://evil.example")
	})

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Empty(t, rec.Result().Cookies())
}

func TestForeignRefererIsRejectedForWrites(t *testing.T) {
	router := testRouter(t, legacyAccountConfig("secret", false))

	rec := postJSON(t, router, "/api/auth/login", `{"username":"alice","password":"secret"}`, func(req *http.Request) {
		req.Header.Set("Referer", "https://evil.example/login")
	})

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestSameOriginWriteIsAllowed(t *testing.T) {
	router := testRouter(t, legacyAccountConfig("secret", false))

	rec := postJSON(t, router, "/api/auth/login", `{"username":"alice","password":"secret"}`, func(req *http.Request) {
		req.Header.Set("Origin", "http://example.com")
	})

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestReadonlyLoginDoesNotRehash(t *testing.T) {
	router, store := testRouterWithPath(t, legacyAccountConfig("secret", true), filepath.Join(t.TempDir(), "paisa.yaml"))

	rec := doLogin(t, router, "alice", "secret")
	require.Equal(t, http.StatusOK, rec.Code)

	stored := store.Get().UserAccounts[0].Password
	assert.True(t, strings.HasPrefix(stored, "sha256:"), "hash should stay legacy in readonly mode, got %q", stored)
}

func TestRehashSaveFailureStillAllowsLogin(t *testing.T) {
	// Bind the store to a directory: Update can mutate the in-memory clone but
	// os.WriteFile fails, so the login must still succeed and keep the old hash.
	dir := t.TempDir()
	router, store := testRouterWithPath(t, legacyAccountConfig("secret", false), dir)

	rec := doLogin(t, router, "alice", "secret")
	require.Equal(t, http.StatusOK, rec.Code)
	require.NotEmpty(t, rec.Result().Cookies())

	stored := store.Get().UserAccounts[0].Password
	assert.True(t, strings.HasPrefix(stored, "sha256:"), "failed save must not publish the rehash, got %q", stored)
}

func TestLoginRehashesLegacyHash(t *testing.T) {
	router, store := testRouterWithPath(t,
		legacyAccountConfig("secret", false),
		filepath.Join(t.TempDir(), "paisa.yaml"))

	rec := doLogin(t, router, "alice", "secret")
	require.Equal(t, http.StatusOK, rec.Code)

	stored := store.Get().UserAccounts[0].Password
	assert.True(t, strings.HasPrefix(stored, "argon2id$"), "expected argon2id hash, got %q", stored)

	ok, rehash, err := utils.VerifyPassword(stored, "secret")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.False(t, rehash)
}

func TestLegacyAuthHeaderIsAcceptedAndMarkedDeprecated(t *testing.T) {
	router := testRouter(t, legacyAccountConfig("secret", false))

	req := httptest.NewRequest(http.MethodGet, "/api/version", nil)
	req.Header.Set("X-Auth", "alice:"+utils.Sha256("secret"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "true", rec.Header().Get("Deprecation"))
	assert.Equal(t, "Sat, 31 Dec 2026 23:59:59 GMT", rec.Header().Get("Sunset"))
}

func TestLegacyAuthHeaderRejectsWrongPassword(t *testing.T) {
	router := testRouter(t, legacyAccountConfig("secret", false))

	req := httptest.NewRequest(http.MethodGet, "/api/version", nil)
	req.Header.Set("X-Auth", "alice:"+utils.Sha256("wrong"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestSecureCookieCanBeForced(t *testing.T) {
	SetSecureCookies(true)
	defer SetSecureCookies(false)

	router := testRouter(t, legacyAccountConfig("secret", false))

	rec := doLogin(t, router, "alice", "secret")
	cookie := requireSessionCookie(t, rec)
	assert.True(t, cookie.Secure)
}

func TestSecureCookieWithTLS(t *testing.T) {
	SetSecureCookies(false)

	router := testRouter(t, legacyAccountConfig("secret", false))

	rec := postJSON(t, router, "/api/auth/login", `{"username":"alice","password":"secret"}`, func(req *http.Request) {
		req.TLS = &tls.ConnectionState{}
	})
	cookie := requireSessionCookie(t, rec)
	assert.True(t, cookie.Secure)
}

func TestCookieIsNotSecureByDefaultOverPlainHTTP(t *testing.T) {
	SetSecureCookies(false)

	router := testRouter(t, legacyAccountConfig("secret", false))

	rec := doLogin(t, router, "alice", "secret")
	cookie := requireSessionCookie(t, rec)
	assert.False(t, cookie.Secure)
}

func TestForgedSessionCookieIsRejected(t *testing.T) {
	router := testRouter(t, legacyAccountConfig("secret", false))

	forged := &http.Cookie{Name: "paisa_session", Value: "made-up"}
	assert.Equal(t, http.StatusUnauthorized, getWithCookie(router, "/api/version", forged).Code)
}

func TestHashEndpointBootstrapAndAuth(t *testing.T) {
	// Bootstrap: no user accounts, hashing is allowed without a session.
	bootstrap := testRouter(t, noAccountConfig)
	rec := postJSON(t, bootstrap, "/api/auth/hash", `{"password":"longenough"}`, nil)
	require.Equal(t, http.StatusOK, rec.Code)

	body := decodeBody(t, rec)
	hash, ok := body["hash"].(string)
	require.True(t, ok)
	assert.True(t, strings.HasPrefix(hash, "argon2id$"))

	verified, _, err := utils.VerifyPassword(hash, "longenough")
	require.NoError(t, err)
	assert.True(t, verified)

	// With user accounts, hashing requires authentication.
	protected := testRouter(t, legacyAccountConfig("secret", false))
	assert.Equal(t, http.StatusUnauthorized,
		postJSON(t, protected, "/api/auth/hash", `{"password":"longenough"}`, nil).Code)

	cookie := requireSessionCookie(t, doLogin(t, protected, "alice", "secret"))
	rec = postJSON(t, protected, "/api/auth/hash", `{"password":"longenough"}`, func(req *http.Request) {
		req.AddCookie(cookie)
	})
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestHashEndpointRejectsShortPassword(t *testing.T) {
	router := testRouter(t, noAccountConfig)

	rec := postJSON(t, router, "/api/auth/hash", `{"password":"short"}`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHashEndpointIsRateLimited(t *testing.T) {
	router := testRouter(t, noAccountConfig)

	rec := postJSON(t, router, "/api/auth/hash", `{"password":"longenough"}`, nil)
	require.Equal(t, http.StatusOK, rec.Code)
	hash, ok := decodeBody(t, rec)["hash"].(string)
	require.True(t, ok)
	assert.True(t, strings.HasPrefix(hash, "argon2id$"))

	// The first call above consumed one slot. Three more fit in the burst,
	// after which the limiter must reject the request before doing Argon2id
	// work again.
	for attempt := 0; attempt < 3; attempt++ {
		require.Equal(t, http.StatusOK,
			postJSON(t, router, "/api/auth/hash", `{"password":"longenough"}`, nil).Code)
	}

	rec = postJSON(t, router, "/api/auth/hash", `{"password":"longenough"}`, nil)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.Contains(t, strings.ToLower(string(rec.Body.Bytes())), "too many hash requests")
}
