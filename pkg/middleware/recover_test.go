package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRegistryRecoveryHandlesPanics(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	RegistryRecovery(engine, nil)
	engine.GET("/panic", func(*gin.Context) {
		panic("boom")
	})

	response := httptest.NewRecorder()
	require.NotPanics(t, func() {
		engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/panic", nil))
	})
	require.Equal(t, http.StatusInternalServerError, response.Code)
}
