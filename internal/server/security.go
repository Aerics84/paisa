package server

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

// cspTemplate is the Content-Security-Policy sent with every response. The
// single %s is the script-src directive value, which is either the hashes of
// the inline bootstrap scripts or the 'unsafe-inline' fallback.
//
// 'unsafe-eval' is required by the user-authored Handlebars templates used by
// the import page: Handlebars compiles them in the browser with new Function.
// Without it the template editor cannot compile and the import preview stays
// empty with only a console error. The rest of the policy still blocks inline
// scripts (apart from the hashed bootstrap), external scripts and framing.
const cspTemplate = "default-src 'self'; script-src 'self' 'unsafe-eval' %s; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'"

var (
	scriptTagPattern = regexp.MustCompile(`(?is)<script\b([^>]*)>(.*?)</script\s*>`)
	// Match src only when it is the whole attribute name, so data-src (or any
	// other *-src attribute) is not treated as an external script source.
	scriptSrcPattern = regexp.MustCompile(`(?i)(?:^|\s)src\s*=`)
)

// BuildCSP returns the Content-Security-Policy header value. Every hash becomes
// a separate 'sha256-...' source. An empty list (inline script extraction
// failed) falls back to 'unsafe-inline' so the application keeps working.
func BuildCSP(scriptHashes []string) string {
	scriptSrc := "'unsafe-inline'"

	sources := make([]string, 0, len(scriptHashes))
	for _, hash := range scriptHashes {
		if hash == "" {
			continue
		}
		sources = append(sources, fmt.Sprintf("'sha256-%s'", hash))
	}
	if len(sources) > 0 {
		scriptSrc = strings.Join(sources, " ")
	}

	return fmt.Sprintf(cspTemplate, scriptSrc)
}

// SecurityHeaders adds hardening headers to every response and allows only the
// hashed inline bootstrap scripts.
func SecurityHeaders(scriptHashes []string) gin.HandlerFunc {
	csp := BuildCSP(scriptHashes)

	return func(c *gin.Context) {
		header := c.Writer.Header()
		header.Set("Content-Security-Policy", csp)
		header.Set("X-Content-Type-Options", "nosniff")
		header.Set("X-Frame-Options", "DENY")
		header.Set("Referrer-Policy", "no-referrer")
		header.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			header.Set("Cache-Control", "no-store")
		}
		c.Next()
	}
}

// extractInlineScriptHashes returns the base64 encoded SHA-256 of every
// non-empty inline <script> in html. Scripts that load external sources via a
// src attribute are ignored. It returns an error when no inline script exists.
//
// The extractor is a deliberately simple regex matcher: it assumes a literal
// "</script>" never appears inside a JavaScript string and that no inline
// script is hidden inside an HTML comment. Unexpected script counts are not
// errors because every discovered inline script is hashed.
func extractInlineScriptHashes(html []byte) ([]string, error) {
	hashes := make([]string, 0, 1)

	for _, match := range scriptTagPattern.FindAllSubmatch(html, -1) {
		attributes := string(match[1])
		content := string(match[2])

		if scriptSrcPattern.MatchString(attributes) {
			continue
		}

		if strings.TrimSpace(content) == "" {
			continue
		}

		sum := sha256.Sum256([]byte(content))
		hashes = append(hashes, base64.StdEncoding.EncodeToString(sum[:]))
	}

	if len(hashes) == 0 {
		return nil, errors.New("no inline script found")
	}

	return hashes, nil
}
