package status

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartStatServer_NoPort(t *testing.T) {
	status := NewStatus()
	status.SetRetention(1 * time.Hour)

	config := &StatServerConfig{
		Port: "",
	}

	server := StartStatServer(status, config)
	assert.Nil(t, server)
}

func TestStartStatServer_WithPort(t *testing.T) {
	status := NewStatus()
	status.SetRetention(1 * time.Hour)

	config := &StatServerConfig{
		Port:      ":0",
		Reports:   []time.Duration{1 * time.Minute},
		Retention: 1 * time.Hour,
	}

	server := StartStatServer(status, config)
	require.NotNil(t, server)
	require.NotNil(t, server.status)
	require.NotNil(t, server.config)

	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	server.StopStatServer(ctx)
}

func TestStatServer_Start_WithTimeouts(t *testing.T) {
	status := NewStatus()
	status.SetRetention(1 * time.Hour)
	status.Update(true)

	config := &StatServerConfig{
		Port:         ":0",
		Reports:      []time.Duration{1 * time.Minute},
		Retention:    1 * time.Hour,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
		IdleTimeout:  2 * time.Second,
	}

	server := &StatServer{
		status: status,
		config: config,
	}

	go server.Start()

	time.Sleep(50 * time.Millisecond)

	require.NotNil(t, server.server)
	assert.Equal(t, 2*time.Second, server.server.ReadTimeout)
	assert.Equal(t, 2*time.Second, server.server.WriteTimeout)
	assert.Equal(t, 2*time.Second, server.server.IdleTimeout)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	server.StopStatServer(ctx)
}

func TestStatServer_Start_UsesDefaultTimeouts(t *testing.T) {
	status := NewStatus()
	status.SetRetention(1 * time.Hour)
	status.Update(true)

	config := &StatServerConfig{
		Port:      ":0",
		Reports:   []time.Duration{1 * time.Minute},
		Retention: 1 * time.Hour,
	}

	server := &StatServer{
		status: status,
		config: config,
	}

	go server.Start()

	time.Sleep(50 * time.Millisecond)

	require.NotNil(t, server.server)
	assert.Equal(t, DefaultStatServerReadTimeout, server.server.ReadTimeout)
	assert.Equal(t, DefaultStatServerWriteTimeout, server.server.WriteTimeout)
	assert.Equal(t, DefaultStatServerIdleTimeout, server.server.IdleTimeout)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	server.StopStatServer(ctx)
}

func TestStopStatServer_NilServer(_ *testing.T) {
	server := &StatServer{
		server: nil,
	}

	ctx := context.Background()
	server.StopStatServer(ctx)
}

func TestStopStatServer_GracefulShutdown(t *testing.T) {
	status := NewStatus()
	status.SetRetention(1 * time.Hour)

	config := &StatServerConfig{
		Port:      ":0",
		Reports:   []time.Duration{1 * time.Minute},
		Retention: 1 * time.Hour,
	}

	server := &StatServer{
		status: status,
		config: config,
	}

	go server.Start()
	time.Sleep(50 * time.Millisecond)

	require.NotNil(t, server.server)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server.StopStatServer(ctx)
}

func TestStatServer_Route(t *testing.T) {
	status := NewStatus()
	status.SetRetention(1 * time.Hour)
	status.Update(true)

	config := &StatServerConfig{
		Port:      ":0",
		Reports:   []time.Duration{1 * time.Minute},
		Retention: 1 * time.Hour,
	}

	server := &StatServer{
		status: status,
		config: config,
	}

	go server.Start()
	time.Sleep(50 * time.Millisecond)

	require.NotNil(t, server.server)

	url := "http://localhost" + server.server.Addr + StatRoute

	client := &http.Client{Timeout: 1 * time.Second}
	resp, err := client.Get(url)
	if err == nil {
		err = resp.Body.Close()
		if err != nil {
			t.Error(err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	server.StopStatServer(ctx)
}

func TestStatServerConfigDefaults(t *testing.T) {
	assert.Equal(t, 3*time.Second, DefaultStatServerReadTimeout)
	assert.Equal(t, 3*time.Second, DefaultStatServerWriteTimeout)
	assert.Equal(t, 3*time.Second, DefaultStatServerIdleTimeout)
	assert.Equal(t, "/stats.json", StatRoute)
}
