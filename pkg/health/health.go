// Package health exposes framework-neutral process health handlers.
package health

import (
	"encoding/json"
	"net/http"
	"sync/atomic"
)

type Gate struct {
	ready atomic.Bool
}

func New() *Gate {
	return &Gate{}
}

func (gate *Gate) SetReady() {
	gate.ready.Store(true)
}

func (gate *Gate) SetNotReady() {
	gate.ready.Store(false)
}

func (gate *Gate) Ready() bool {
	return gate.ready.Load()
}

// LivenessHandler reports only that the process can serve requests. Dependency
// availability must not be used as a liveness signal.
func (gate *Gate) LivenessHandler() http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writeStatus(writer, http.StatusOK, "ok")
	})
}

// ReadinessHandler reports whether the application owner currently accepts
// traffic. A new Gate starts not ready.
func (gate *Gate) ReadinessHandler() http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		if gate.Ready() {
			writeStatus(writer, http.StatusOK, "ready")
			return
		}
		writeStatus(writer, http.StatusServiceUnavailable, "not_ready")
	})
}

func writeStatus(writer http.ResponseWriter, status int, value string) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(map[string]string{"status": value})
}
