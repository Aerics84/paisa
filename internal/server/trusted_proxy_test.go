package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestBuildIgnoresForwardedForHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := Build(Deps{DB: nil, Config: config.DefaultStore()}, false)
	router.GET("/test-client-ip", func(c *gin.Context) {
		c.String(http.StatusOK, c.ClientIP())
	})

	request := httptest.NewRequest(http.MethodGet, "/test-client-ip", nil)
	request.RemoteAddr = "203.0.113.9:1234"
	request.Header.Set("X-Forwarded-For", "10.0.0.1")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	assert.Equal(t, "203.0.113.9", response.Body.String())
}
