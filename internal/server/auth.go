package server

import (
	"crypto/subtle"
	"math"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/utils"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"github.com/throttled/throttled/v2"
	"github.com/throttled/throttled/v2/store/memstore"
)

const (
	sessionCookieName = "paisa_session"
	sessionTTL        = 24 * time.Hour

	// legacyAuthSunset is the announced removal date for the X-Auth header.
	legacyAuthSunset = "Sat, 31 Dec 2026 23:59:59 GMT"

	minPasswordLength = 8

	// authUsernameKey stores the authenticated username in the gin context.
	authUsernameKey = "paisa.auth.username"
)

// secureCookies forces the Secure attribute on session cookies. It is enabled
// by --secure-cookies / PAISA_SECURE_COOKIES for deployments behind a TLS
// terminating reverse proxy, where the request itself arrives over plain HTTP.
var secureCookies atomic.Bool

// SetSecureCookies controls whether session cookies always carry Secure.
func SetSecureCookies(enabled bool) {
	secureCookies.Store(enabled)
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type hashRequest struct {
	Password string `json:"password"`
}

// HandleLogin authenticates a user against deps.Config and issues an in-memory
// session cookie. Successful legacy (sha256) logins are transparently upgraded
// to Argon2id when the configuration is writable.
func HandleLogin(deps Deps) gin.HandlerFunc {
	// Login failures are throttled independently of the general API limiter,
	// because /api/auth/login is exempt from session authentication.
	rateLimiter := newAuthRateLimiter()

	return func(c *gin.Context) {
		limitKey := rateLimitKey(c)

		if _, detail, err := rateLimiter.RateLimitCtx(c.Request.Context(), limitKey, 0); err == nil && detail.Remaining <= 0 {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "Too many login attempts. Try again later."})
			return
		}

		failed := func() {
			rateLimiter.RateLimitCtx(c.Request.Context(), limitKey, 1)
		}

		var request loginRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		cfg := deps.Config.Get()
		account, found := findUserAccount(cfg.UserAccounts, request.Username)
		if !found || request.Password == "" {
			failed()
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
			return
		}

		ok, needsRehash, err := utils.VerifyPassword(account.Password, request.Password)
		if err != nil || !ok {
			failed()
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
			return
		}

		if needsRehash && !cfg.Readonly {
			rehashErr := deps.Config.Update(func(next *config.Config) error {
				hash, hashErr := utils.HashPassword(request.Password)
				if hashErr != nil {
					return hashErr
				}

				for i := range next.UserAccounts {
					if next.UserAccounts[i].Username == request.Username {
						next.UserAccounts[i].Password = hash
						break
					}
				}

				return nil
			})
			if rehashErr != nil {
				// The login must not fail because the upgrade could not be
				// persisted; it will be retried on the next login.
				log.Warnf("could not rehash password for %q: %v", request.Username, rehashErr)
			}
		}

		token, expires := deps.Auth.Create(request.Username)
		http.SetCookie(c.Writer, newSessionCookie(token, cookieMaxAge(expires), c.Request))

		c.JSON(http.StatusOK, gin.H{"success": true, "username": request.Username})
	}
}

// HandleLogout deletes the current session and clears the session cookie.
func HandleLogout(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if token, err := c.Cookie(sessionCookieName); err == nil && token != "" {
			deps.Auth.Delete(token)
		}

		cookie := newSessionCookie("", -1, c.Request)
		http.SetCookie(c.Writer, cookie)

		c.JSON(http.StatusOK, gin.H{"success": true})
	}
}

// HandleAuthStatus reports whether authentication is configured and whether
// the current request carries a valid session.
func HandleAuthStatus(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := deps.Config.Get()

		authenticated := false
		username := ""
		if token, err := c.Cookie(sessionCookieName); err == nil && token != "" {
			if session, ok := deps.Auth.Lookup(token); ok {
				authenticated = true
				username = session.Username
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"auth_enabled":  len(cfg.UserAccounts) > 0,
			"authenticated": authenticated,
			"username":      username,
		})
	}
}

// HandleHash hashes a password with Argon2id. It is reachable without a
// session during bootstrap (no user accounts configured); otherwise the auth
// middleware requires a valid session or the deprecated X-Auth header.
func HandleHash(deps Deps) gin.HandlerFunc {
	// Hashing is CPU/memory intensive. Bootstrap mode allows unauthenticated
	// access, so cap per-IP usage independently of the general API limiter.
	rateLimiter := newAuthRateLimiter()

	return func(c *gin.Context) {
		limitKey := rateLimitKey(c)
		if _, detail, err := rateLimiter.RateLimitCtx(c.Request.Context(), limitKey, 0); err == nil && detail.Remaining <= 0 {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "Too many hash requests. Try again later."})
			return
		}

		cfg := deps.Config.Get()
		if len(cfg.UserAccounts) > 0 {
			if _, ok := c.Get(authUsernameKey); !ok {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid Token"})
				return
			}
		}

		var request hashRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		if len(request.Password) < minPasswordLength {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "password must be at least 8 characters",
			})
			return
		}

		rateLimiter.RateLimitCtx(c.Request.Context(), limitKey, 1)

		hash, err := utils.HashPassword(request.Password)
		if err != nil {
			// Never include the password or the hash in the error response.
			log.Errorf("could not hash password: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not hash password"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"hash": hash})
	}
}

func findUserAccount(accounts []config.UserAccount, username string) (config.UserAccount, bool) {
	for _, account := range accounts {
		if subtle.ConstantTimeCompare([]byte(account.Username), []byte(username)) == 1 {
			return account, true
		}
	}

	return config.UserAccount{}, false
}

func newSessionCookie(token string, maxAge int, request *http.Request) *http.Cookie {
	secure := secureCookies.Load()
	if request != nil && request.TLS != nil {
		secure = true
	}

	return &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   secure,
	}
}

func cookieMaxAge(expires time.Time) int {
	maxAge := int(math.Ceil(time.Until(expires).Seconds()))
	if maxAge <= 0 {
		maxAge = int(sessionTTL.Seconds())
	}

	return maxAge
}

// CSRFMiddleware rejects cross-origin state-changing requests below /api.
// Requests without an Origin or Referer header (CLI clients, tests) are
// allowed. No CORS response headers are emitted: foreign origins are simply
// refused rather than granted access.
func CSRFMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		default:
			c.Next()
			return
		}

		if !strings.HasPrefix(c.Request.URL.Path, "/api") {
			c.Next()
			return
		}

		source := c.Request.Header.Get("Origin")
		if source == "" {
			source = c.Request.Header.Get("Referer")
		}
		if source == "" {
			c.Next()
			return
		}

		parsed, err := url.Parse(source)
		if err != nil || parsed.Host == "" || !strings.EqualFold(parsed.Host, c.Request.Host) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "cross-origin request rejected"})
			return
		}

		c.Next()
	}
}

func newAuthRateLimiter() throttled.RateLimiterCtx {
	store, err := memstore.NewCtx(10)
	if err != nil {
		log.Fatal(err)
	}

	quota := throttled.RateQuota{
		MaxRate:  throttled.PerMin(6),
		MaxBurst: 3,
	}

	rateLimiter, err := throttled.NewGCRARateLimiterCtx(store, quota)
	if err != nil {
		log.Fatal(err)
	}

	return rateLimiter
}

// TokenAuthMiddleware authenticates API requests using the paisa_session
// cookie. It accepts the legacy X-Auth header for one release and marks such
// responses as deprecated.
func TokenAuthMiddleware(deps Deps) gin.HandlerFunc {
	rateLimiter := newAuthRateLimiter()

	return func(c *gin.Context) {
		cfg := deps.Config.Get()
		if len(cfg.UserAccounts) == 0 || !strings.HasPrefix(c.Request.URL.Path, "/api") {
			c.Next()
			return
		}

		// Public paths are exempt from the general limiter. In particular, the
		// SPA can fire many parallel requests when a session expires; those 401s
		// must not consume the budget that /api/auth/login needs.
		if isPublicAuthPath(c.Request.URL.Path) {
			c.Next()
			return
		}

		_, detail, _ := rateLimiter.RateLimitCtx(c.Request.Context(), rateLimitKey(c), 0)
		if detail.Remaining <= 0 {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "Too many requests"})
			return
		}

		if token, err := c.Cookie(sessionCookieName); err == nil && token != "" {
			if session, ok := deps.Auth.Lookup(token); ok {
				c.Set(authUsernameKey, session.Username)
				c.Next()
				return
			}
		}

		if username, ok := legacyAuthHeaderUser(c.Request.Header.Get("X-Auth"), cfg.UserAccounts); ok {
			c.Header("Deprecation", "true")
			c.Header("Sunset", legacyAuthSunset)
			log.Warnf("accepted deprecated X-Auth header for %q; it will be removed in 0.10.0", username)
			c.Set(authUsernameKey, username)
			c.Next()
			return
		}

		rateLimiter.RateLimitCtx(c.Request.Context(), rateLimitKey(c), 1)
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
	}
}

func isPublicAuthPath(path string) bool {
	switch path {
	case "/api/ping", "/api/auth/login", "/api/auth/logout", "/api/auth/status":
		return true
	default:
		return false
	}
}

// legacyAuthHeaderUser validates the deprecated X-Auth header, whose value is
// "username:sha256(password)" (a single SHA-256 over the UTF-8 password).
func legacyAuthHeaderUser(header string, accounts []config.UserAccount) (string, bool) {
	tokens := strings.SplitN(header, ":", 2)
	if len(tokens) != 2 || tokens[0] == "" || tokens[1] == "" {
		return "", false
	}

	hashed := "sha256:" + utils.Sha256(tokens[1])
	for _, account := range accounts {
		if subtle.ConstantTimeCompare([]byte(account.Username), []byte(tokens[0])) == 1 &&
			subtle.ConstantTimeCompare([]byte(account.Password), []byte(hashed)) == 1 {
			return account.Username, true
		}
	}

	return "", false
}
