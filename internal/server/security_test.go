package server

import (
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func sha256Base64(content string) string {
	sum := sha256.Sum256([]byte(content))
	return base64.StdEncoding.EncodeToString(sum[:])
}

func TestSecurityHeaders(t *testing.T) {
	router := testRouter(t, "journal_path: main.ledger\ndb_path: paisa.db\n")
	req := httptest.NewRequest(http.MethodGet, "/api/ping", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	require.Equal(t, "DENY", rec.Header().Get("X-Frame-Options"))
	require.Equal(t, "no-referrer", rec.Header().Get("Referrer-Policy"))
	require.Equal(t, "camera=(), microphone=(), geolocation=()", rec.Header().Get("Permissions-Policy"))
	require.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	require.Contains(t, rec.Header().Get("Content-Security-Policy"), "default-src 'self'")
	require.Contains(t, rec.Header().Get("Content-Security-Policy"), "frame-ancestors 'none'")
	require.Contains(t, rec.Header().Get("Content-Security-Policy"), "script-src 'self' 'sha256-")

	// The HTML document (NoRoute serves the embedded index) must carry the same
	// policy so its inline bootstrap script is explicitly allowed.
	indexReq := httptest.NewRequest(http.MethodGet, "/", nil)
	indexRec := httptest.NewRecorder()
	router.ServeHTTP(indexRec, indexReq)

	require.Equal(t, http.StatusOK, indexRec.Code)
	require.Equal(t, rec.Header().Get("Content-Security-Policy"), indexRec.Header().Get("Content-Security-Policy"))
	require.Contains(t, indexRec.Header().Get("Content-Security-Policy"), "script-src 'self' 'sha256-")
}

func TestBuildCSPIncludesScriptHash(t *testing.T) {
	require.Equal(t,
		"default-src 'self'; script-src 'self' 'sha256-abc123'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'",
		BuildCSP([]string{"abc123"}),
	)

	csp := BuildCSP([]string{"abc123"})
	require.Contains(t, csp, "style-src 'self' 'unsafe-inline'")
	require.Contains(t, csp, "base-uri 'self'")
	require.Contains(t, csp, "form-action 'self'")

	fallback := BuildCSP(nil)
	require.Contains(t, fallback, "script-src 'self' 'unsafe-inline'")
	require.NotContains(t, fallback, "sha256-")

	// An empty list behaves like the no-hash fallback.
	require.Equal(t, fallback, BuildCSP([]string{}))
}

func TestExtractInlineScriptHash(t *testing.T) {
	script := `console.log("x")`
	hashes, err := extractInlineScriptHashes([]byte(`<html><script>` + script + `</script></html>`))
	require.NoError(t, err)
	require.Equal(t, []string{sha256Base64(script)}, hashes)

	_, err = extractInlineScriptHashes([]byte(`<html><script src="/app.js"></script></html>`))
	require.Error(t, err)

	_, err = extractInlineScriptHashes([]byte(`<html><body>no script</body></html>`))
	require.Error(t, err)

	// data-src is not src: the script is inline and must be hashed.
	hashes, err = extractInlineScriptHashes([]byte(`<html><script data-src="/app.js">y</script></html>`))
	require.NoError(t, err)
	require.Equal(t, []string{sha256Base64("y")}, hashes)
}

func TestExtractInlineScriptHashesMultiple(t *testing.T) {
	first := `console.log("one")`
	second := `console.log("two")`
	html := `<html><script>` + first + `</script><script src="/app.js"></script><script type="module">` + second + `</script></html>`

	hashes, err := extractInlineScriptHashes([]byte(html))
	require.NoError(t, err)
	require.Equal(t, []string{sha256Base64(first), sha256Base64(second)}, hashes)

	csp := BuildCSP(hashes)
	require.Contains(t, csp, "'sha256-"+sha256Base64(first)+"'")
	require.Contains(t, csp, "'sha256-"+sha256Base64(second)+"'")
	require.NotContains(t, csp, "script-src 'self' 'unsafe-inline'")
	require.True(t, strings.Contains(csp, "script-src 'self' 'sha256-"+sha256Base64(first)+"' 'sha256-"+sha256Base64(second)+"'"))
}
