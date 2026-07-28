package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCORSRejectsWildcard(t *testing.T) {
	_, err := NewCORS(CORSConfig{AllowOrigins: []string{"*"}})
	require.Error(t, err)
}

func TestCORSUsesExplicitAllowlist(t *testing.T) {
	cors, err := NewCORS(CORSConfig{AllowOrigins: []string{"https://app.example.com"}, AllowCredentials: true})
	require.NoError(t, err)
	engine := gin.New()
	engine.Use(cors)
	engine.GET("/", func(ctx *gin.Context) { ctx.Status(http.StatusOK) })

	allowed := httptest.NewRequest(http.MethodGet, "/", nil)
	allowed.Header.Set("Origin", "https://app.example.com")
	allowedResponse := httptest.NewRecorder()
	engine.ServeHTTP(allowedResponse, allowed)
	require.Equal(t, "https://app.example.com", allowedResponse.Header().Get("Access-Control-Allow-Origin"))
	require.ElementsMatch(t,
		[]string{"Origin", "Access-Control-Request-Method", "Access-Control-Request-Headers"},
		allowedResponse.Header().Values("Vary"),
	)

	denied := httptest.NewRequest(http.MethodGet, "/", nil)
	denied.Header.Set("Origin", "https://evil.example.com")
	deniedResponse := httptest.NewRecorder()
	engine.ServeHTTP(deniedResponse, denied)
	require.Equal(t, http.StatusForbidden, deniedResponse.Code)
}

func TestCORSRejectsDisallowedPreflightMethodAndHeader(t *testing.T) {
	cors, err := NewCORS(CORSConfig{
		AllowOrigins: []string{"https://app.example.com"},
		AllowMethods: []string{http.MethodGet},
		AllowHeaders: []string{"Content-Type"},
	})
	require.NoError(t, err)
	engine := gin.New()
	engine.Use(cors)
	engine.OPTIONS("/", func(ctx *gin.Context) { ctx.Status(http.StatusNoContent) })

	for _, configure := range []func(*http.Request){
		func(request *http.Request) {
			request.Header.Set("Access-Control-Request-Method", http.MethodDelete)
		},
		func(request *http.Request) {
			request.Header.Set("Access-Control-Request-Method", http.MethodGet)
			request.Header.Set("Access-Control-Request-Headers", "X-Admin-Token")
		},
	} {
		request := httptest.NewRequest(http.MethodOptions, "/", nil)
		request.Header.Set("Origin", "https://app.example.com")
		configure(request)
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, request)
		require.Equal(t, http.StatusForbidden, response.Code)
	}
}

func TestGzipCompressesRegularResponsesAndSkipsSSE(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(GzipMiddleware())
	engine.GET("/json", func(ctx *gin.Context) { ctx.JSON(http.StatusOK, gin.H{"value": "ok"}) })
	engine.GET("/sse", func(ctx *gin.Context) {
		ctx.Header("Content-Type", "text/event-stream")
		_, _ = ctx.Writer.WriteString("data: event\n\n")
		ctx.Writer.Flush()
	})

	jsonRequest := httptest.NewRequest(http.MethodGet, "/json", nil)
	jsonRequest.Header.Set("Accept-Encoding", "gzip")
	jsonResponse := httptest.NewRecorder()
	engine.ServeHTTP(jsonResponse, jsonRequest)
	require.Equal(t, "gzip", jsonResponse.Header().Get("Content-Encoding"))
	compressed, err := gzip.NewReader(jsonResponse.Body)
	require.NoError(t, err)
	body, err := io.ReadAll(compressed)
	require.NoError(t, err)
	require.Contains(t, string(body), `"value":"ok"`)

	sseRequest := httptest.NewRequest(http.MethodGet, "/sse", nil)
	sseRequest.Header.Set("Accept-Encoding", "gzip")
	sseResponse := httptest.NewRecorder()
	engine.ServeHTTP(sseResponse, sseRequest)
	require.Empty(t, sseResponse.Header().Get("Content-Encoding"))
	require.Equal(t, "data: event\n\n", sseResponse.Body.String())
}
