package server

import (
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSecurityHeaders(t *testing.T) {
	router := testRouter(t, "journal_path: main.ledger\ndb_path: paisa.db\n")
	req := httptest.NewRequest(http.MethodGet, "/api/ping", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	require.Equal(t, "DENY", rec.Header().Get("X-Frame-Options"))
	require.Equal(t, "no-referrer", rec.Header().Get("Referrer-Policy"))
	require.Equal(t, "camera=(), microphone=(), geolocation=()", rec.Header().Get("Permissions-Policy"))
	require.Contains(t, rec.Header().Get("Content-Security-Policy"), "default-src 'self'")
	require.Contains(t, rec.Header().Get("Content-Security-Policy"), "frame-ancestors 'none'")
	require.Contains(t, rec.Header().Get("Content-Security-Policy"), "script-src 'self' 'sha256-")
}

func TestBuildCSPIncludesScriptHash(t *testing.T) {
	csp := BuildCSP("abc123")
	require.Contains(t, csp, "script-src 'self' 'sha256-abc123'")
	require.Contains(t, csp, "style-src 'self' 'unsafe-inline'")
	require.Contains(t, csp, "base-uri 'self'")
	require.Contains(t, csp, "form-action 'self'")

	fallback := BuildCSP("")
	require.Contains(t, fallback, "script-src 'self' 'unsafe-inline'")
	require.NotContains(t, fallback, "sha256-")
}

func TestExtractInlineScriptHash(t *testing.T) {
	script := `console.log("x")`
	hash, err := extractInlineScriptHash([]byte(`<html><script>` + script + `</script></html>`))
	require.NoError(t, err)

	sum := sha256.Sum256([]byte(script))
	require.Equal(t, base64.StdEncoding.EncodeToString(sum[:]), hash)

	_, err = extractInlineScriptHash([]byte(`<html><script src="/app.js"></script></html>`))
	require.Error(t, err)

	_, err = extractInlineScriptHash([]byte(`<html><body>no script</body></html>`))
	require.Error(t, err)
}
