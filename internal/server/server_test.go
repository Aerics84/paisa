package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRateLimitKeyUsesClientIP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/ping", nil)
	ctx.Request.RemoteAddr = "203.0.113.9:1234"
	assert.Equal(t, "203.0.113.9", rateLimitKey(ctx))
}
