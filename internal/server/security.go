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
// single %s is the script-src directive value, which is either a hash of the
// inline bootstrap script or the 'unsafe-inline' fallback.
const cspTemplate = "default-src 'self'; script-src 'self' %s; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'"

var (
	scriptTagPattern = regexp.MustCompile(`(?is)<script\b([^>]*)>(.*?)</script\s*>`)
	scriptSrcPattern = regexp.MustCompile(`(?i)\bsrc\s*=`)
)

// BuildCSP returns the Content-Security-Policy header value. When scriptHash
// is empty (inline script extraction failed), it falls back to 'unsafe-inline'
// so the application keeps working.
func BuildCSP(scriptHash string) string {
	scriptSrc := "'unsafe-inline'"
	if scriptHash != "" {
		scriptSrc = fmt.Sprintf("'sha256-%s'", scriptHash)
	}

	return fmt.Sprintf(cspTemplate, scriptSrc)
}

// SecurityHeaders adds hardening headers to every response and allows only the
// hashed inline bootstrap script.
func SecurityHeaders(scriptHash string) gin.HandlerFunc {
	csp := BuildCSP(scriptHash)

	return func(c *gin.Context) {
		header := c.Writer.Header()
		header.Set("Content-Security-Policy", csp)
		header.Set("X-Content-Type-Options", "nosniff")
		header.Set("X-Frame-Options", "DENY")
		header.Set("Referrer-Policy", "no-referrer")
		header.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		c.Next()
	}
}

// extractInlineScriptHash returns the base64 encoded SHA-256 of the first
// non-empty inline <script> in html. Scripts that load external sources via a
// src attribute are ignored. It returns an error when no inline script exists.
func extractInlineScriptHash(html []byte) (string, error) {
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
		return base64.StdEncoding.EncodeToString(sum[:]), nil
	}

	return "", errors.New("no inline script found")
}
