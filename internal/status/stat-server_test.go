package status

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hugoh/upd/internal/version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func startStatServer(t *testing.T, config *StatServerConfig) *StatServer {
	t.Helper()

	status := NewStatus()
	status.SetRetention(1 * time.Hour)
	server := StartStatServer(status, config)
	require.NotNil(t, server)

	time.Sleep(50 * time.Millisecond)

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()

		server.Shutdown(ctx)
	})

	return server
}

func TestStartStatServer_NoPort(t *testing.T) {
	status := NewStatus()
	status.SetRetention(1 * time.Hour)

	config := &StatServerConfig{
		Port: 0,
	}

	server := StartStatServer(status, config)
	assert.Nil(t, server)
}

func TestStartStatServer_WithPort(t *testing.T) {
	config := &StatServerConfig{
		Port:      18765,
		Reports:   []time.Duration{time.Minute},
		Retention: time.Hour,
	}

	server := startStatServer(t, config)
	require.NotNil(t, server.status)
	require.NotNil(t, server.config)
}

func TestStatServer_Start_WithTimeouts(t *testing.T) {
	config := &StatServerConfig{
		Port:         18765,
		Reports:      []time.Duration{time.Minute},
		Retention:    time.Hour,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
		IdleTimeout:  2 * time.Second,
	}

	server := startStatServer(t, config)
	require.NotNil(t, server.server)
	assert.Equal(t, 2*time.Second, server.server.ReadTimeout)
	assert.Equal(t, 2*time.Second, server.server.WriteTimeout)
	assert.Equal(t, 2*time.Second, server.server.IdleTimeout)
}

func TestStatServer_Start_UsesDefaultTimeouts(t *testing.T) {
	config := &StatServerConfig{
		Port:      18765,
		Reports:   []time.Duration{time.Minute},
		Retention: time.Hour,
	}

	server := startStatServer(t, config)
	require.NotNil(t, server.server)
	assert.Equal(t, DefaultStatServerReadTimeout, server.server.ReadTimeout)
	assert.Equal(t, DefaultStatServerWriteTimeout, server.server.WriteTimeout)
	assert.Equal(t, DefaultStatServerIdleTimeout, server.server.IdleTimeout)
}

func TestShutdown_NilServer(t *testing.T) {
	server := &StatServer{
		server: nil,
	}

	ctx := t.Context()
	server.Shutdown(ctx)
}

func TestShutdown_GracefulShutdown(t *testing.T) {
	config := &StatServerConfig{
		Port:      18765,
		Reports:   []time.Duration{time.Minute},
		Retention: time.Hour,
	}

	server := startStatServer(t, config)
	require.NotNil(t, server.server)
}

func TestStatServer_Route(t *testing.T) {
	config := &StatServerConfig{
		Port:      18765,
		Reports:   []time.Duration{time.Minute},
		Retention: time.Hour,
	}

	server := startStatServer(t, config)
	require.NotNil(t, server.server)

	url := "http://localhost" + server.server.Addr + StatRoute

	client := &http.Client{Timeout: 1 * time.Second}

	resp, err := client.Get(url)
	require.NoError(t, err)

	t.Cleanup(func() { _ = resp.Body.Close() })

	assert.Equal(t, "upd/"+version.Version(), resp.Header.Get("Server"))
}

func TestStatServerConfigDefaults(t *testing.T) {
	assert.Equal(t, 3*time.Second, DefaultStatServerReadTimeout)
	assert.Equal(t, 3*time.Second, DefaultStatServerWriteTimeout)
	assert.Equal(t, 3*time.Second, DefaultStatServerIdleTimeout)
	assert.Equal(t, "/stats.json", StatRoute)
}

func TestDefaultTimeout(t *testing.T) {
	assert.Equal(t, 5*time.Second, defaultTimeout(0, 5*time.Second))
	assert.Equal(t, 3*time.Second, defaultTimeout(3*time.Second, 5*time.Second))
}

func TestServerHeader_SetsHeaderOnSuccessfulRoute(t *testing.T) {
	handler := serverHeader(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, "upd/"+version.Version(), rec.Header().Get("Server"))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "OK", rec.Body.String())
}

func TestServerHeader_SetsHeaderOnNonExistentRoute(t *testing.T) {
	handler := serverHeader(http.NewServeMux())

	req := httptest.NewRequest(http.MethodGet, "/nonexistent", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, "upd/"+version.Version(), rec.Header().Get("Server"))
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestServerHeader_SetsHeaderOnMethodNotAllowed(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("GET "+StatRoute, &StatHandler{statServer: &StatServer{}})
	handler := serverHeader(mux)

	req := httptest.NewRequest(http.MethodPost, StatRoute, http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, "upd/"+version.Version(), rec.Header().Get("Server"))
	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}
