package health

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReadinessStartsUnavailableAndCanTransition(t *testing.T) {
	gate := &Gate{}

	response := httptest.NewRecorder()
	gate.ReadinessHandler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/ready", nil))
	require.Equal(t, http.StatusServiceUnavailable, response.Code)

	gate.SetReady()
	response = httptest.NewRecorder()
	gate.ReadinessHandler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/ready", nil))
	require.Equal(t, http.StatusOK, response.Code)

	gate.SetNotReady()
	response = httptest.NewRecorder()
	gate.ReadinessHandler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/ready", nil))
	require.Equal(t, http.StatusServiceUnavailable, response.Code)
}

func TestLivenessDoesNotDependOnReadiness(t *testing.T) {
	gate := &Gate{}
	response := httptest.NewRecorder()

	gate.LivenessHandler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/live", nil))

	require.Equal(t, http.StatusOK, response.Code)
	require.JSONEq(t, `{"status":"ok"}`, response.Body.String())
}
