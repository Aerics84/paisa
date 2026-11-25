package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/web"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// staticRouter builds the real router so the tests exercise the NoRoute handler
// that serves root-level files from the embedded web/static tree.
func staticRouter(t *testing.T) *gin.Engine {
	t.Helper()

	gin.SetMode(gin.TestMode)
	return Build(Deps{Config: config.NewStore()}, false)
}

func getStatic(t *testing.T, router *gin.Engine, target string) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(http.MethodGet, target, nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func TestStaticRootServing(t *testing.T) {
	router := staticRouter(t)

	tests := []struct {
		name            string
		target          string
		contentType     string
		noCache         bool
		bodyStartsWith  string
		bodyEqualsIndex bool
	}{
		{
			name:           "service worker is javascript and not cached",
			target:         "/sw.js",
			contentType:    "javascript",
			noCache:        true,
			bodyStartsWith: "const CACHE_PREFIX",
		},
		{
			name:           "manifest uses the webmanifest media type",
			target:         "/manifest.webmanifest",
			contentType:    "application/manifest+json",
			noCache:        true,
			bodyStartsWith: "{",
		},
		{
			name:        "icon is served as png",
			target:      "/icons/icon-192.png",
			contentType: "image/png",
		},
		{
			name:            "unknown app route falls back to the SPA shell",
			target:          "/ledger/import",
			contentType:     "text/html",
			bodyEqualsIndex: true,
		},
		{
			name:            "parent traversal is not served",
			target:          "/../web.go",
			contentType:     "text/html",
			bodyEqualsIndex: true,
		},
		{
			name:            "encoded parent traversal is not served",
			target:          "/%2e%2e/web.go",
			contentType:     "text/html",
			bodyEqualsIndex: true,
		},
		{
			name:            "traversal out of icons is not served",
			target:          "/icons/../../secret",
			contentType:     "text/html",
			bodyEqualsIndex: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := getStatic(t, router, tt.target)

			require.Equal(t, http.StatusOK, response.Code)
			assert.Contains(t, response.Header().Get("Content-Type"), tt.contentType)
			assert.Equal(t, tt.noCache, response.Header().Get("Cache-Control") == "no-cache")

			body := response.Body.String()
			if tt.bodyEqualsIndex {
				assert.Equal(t, web.Index, body)
				assert.NotContains(t, body, "package web")
			}
			if tt.bodyStartsWith != "" {
				assert.True(
					t,
					strings.HasPrefix(body, tt.bodyStartsWith),
					"body starts with %q",
					body[:min(len(body), 40)],
				)
			}
		})
	}
}
