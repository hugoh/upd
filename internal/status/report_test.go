package status

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testPort = ":8080"

func TestNewStatHandler(t *testing.T) {
	status := NewStatus()
	status.SetRetention(1 * time.Hour)

	config := &StatServerConfig{
		Port:      testPort,
		Reports:   []time.Duration{1 * time.Minute},
		Retention: 1 * time.Hour,
	}

	server := &StatServer{
		status: status,
		config: config,
	}

	handler := NewStatHandler(server)
	require.NotNil(t, handler)
	assert.Equal(t, server, handler.statServer)
}

func TestStatHandler_GenStatReport(t *testing.T) {
	status := NewStatus()
	status.SetRetention(1 * time.Hour)
	status.Update(true)

	config := &StatServerConfig{
		Port:      testPort,
		Reports:   []time.Duration{1 * time.Minute, 5 * time.Minute},
		Retention: 1 * time.Hour,
	}

	server := &StatServer{
		status: status,
		config: config,
	}

	handler := NewStatHandler(server)

	report := handler.GenStatReport()
	require.NotNil(t, report)
	assert.True(t, report.Up)
	assert.Len(t, report.Stats, 2)
	assert.NotEmpty(t, report.Version)
	assert.False(t, report.Generated.IsZero())
}

func TestStatHandler_GenStatReport_WithChanges(t *testing.T) {
	status := NewStatus()
	status.SetRetention(1 * time.Hour)
	status.Update(true)
	time.Sleep(10 * time.Millisecond)
	status.Update(false)
	time.Sleep(10 * time.Millisecond)
	status.Update(true)

	config := &StatServerConfig{
		Port:      testPort,
		Reports:   []time.Duration{1 * time.Minute},
		Retention: 1 * time.Hour,
	}

	server := &StatServer{
		status: status,
		config: config,
	}

	handler := NewStatHandler(server)

	report := handler.GenStatReport()
	require.NotNil(t, report)
	assert.True(t, report.Up)
	assert.GreaterOrEqual(t, report.CheckCount, int64(3))
}

func TestStatHandler_ServeHTTP(t *testing.T) {
	status := NewStatus()
	status.SetRetention(1 * time.Hour)
	status.Update(true)

	config := &StatServerConfig{
		Port:      testPort,
		Reports:   []time.Duration{1 * time.Minute},
		Retention: 1 * time.Hour,
	}

	server := &StatServer{
		status: status,
		config: config,
	}

	handler := NewStatHandler(server)

	req := httptest.NewRequest(http.MethodGet, StatRoute, http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var raw map[string]any

	err := json.Unmarshal(rec.Body.Bytes(), &raw)
	require.NoError(t, err)
	assert.Equal(t, true, raw["isUp"])
	assert.NotEmpty(t, raw["updVersion"])
}

func TestStatHandler_ServeHTTP_MethodNotAllowed(t *testing.T) {
	status := NewStatus()
	status.SetRetention(1 * time.Hour)
	status.Update(true)

	config := &StatServerConfig{
		Port:      testPort,
		Reports:   []time.Duration{1 * time.Minute},
		Retention: 1 * time.Hour,
	}

	server := &StatServer{
		status: status,
		config: config,
	}

	handler := NewStatHandler(server)

	req := httptest.NewRequest(http.MethodPost, StatRoute, http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestStatHandler_ServeHTTP_MethodHead(t *testing.T) {
	status := NewStatus()
	status.SetRetention(1 * time.Hour)
	status.Update(true)

	config := &StatServerConfig{
		Port:      testPort,
		Reports:   []time.Duration{1 * time.Minute},
		Retention: 1 * time.Hour,
	}

	server := &StatServer{
		status: status,
		config: config,
	}

	handler := NewStatHandler(server)

	req := httptest.NewRequest(http.MethodHead, StatRoute, http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestStatHandler_ServeHTTP_JSONFormat(t *testing.T) {
	status := NewStatus()
	status.SetRetention(1 * time.Hour)
	status.Update(true)

	config := &StatServerConfig{
		Port:      testPort,
		Reports:   []time.Duration{1 * time.Minute},
		Retention: 1 * time.Hour,
	}

	server := &StatServer{
		status: status,
		config: config,
	}

	handler := NewStatHandler(server)

	req := httptest.NewRequest(http.MethodGet, StatRoute, http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	body := rec.Body.String()
	assert.Contains(t, body, `"isUp": true`)
	assert.Contains(t, body, `"reports"`)
	assert.Contains(t, body, `"totalChecksRun"`)
	assert.Contains(t, body, `"updVersion"`)
}

func TestReportByPeriod_JSON_Marshal(t *testing.T) {
	report := ReportByPeriod{
		Period:       ReadableDuration(1 * time.Minute),
		Availability: ReadablePercent(99.9),
		Downtime:     ReadableDuration(6 * time.Second),
	}

	data, err := json.Marshal(report)
	require.NoError(t, err)

	assert.Contains(t, string(data), `"period":`)
	assert.Contains(t, string(data), `"availability":`)
	assert.Contains(t, string(data), `"downTime":`)
}

func TestReport_JSON_Marshal(t *testing.T) {
	report := &Report{
		Up:         true,
		Stats:      []ReportByPeriod{{Period: ReadableDuration(1 * time.Minute)}},
		CheckCount: 100,
		LastUpdate: ReadableDuration(5 * time.Second),
		Uptime:     ReadableDuration(1 * time.Hour),
		Version:    "1.0.0",
		Generated:  time.Now(),
	}

	data, err := json.MarshalIndent(report, "", JSONIndentSpaces)
	require.NoError(t, err)

	var raw map[string]any

	err = json.Unmarshal(data, &raw)
	require.NoError(t, err)

	assert.Equal(t, true, raw["isUp"])
	assert.InEpsilon(t, float64(100), raw["totalChecksRun"], 0.01)
	assert.Equal(t, "1.0.0", raw["updVersion"])
}

func TestReport_JSONFieldNames(t *testing.T) {
	report := &Report{
		Up:         true,
		Stats:      []ReportByPeriod{},
		CheckCount: 42,
		LastUpdate: ReadableDuration(10 * time.Second),
		Uptime:     ReadableDuration(1 * time.Hour),
		Version:    "test-version",
		Generated:  time.Now(),
	}

	data, err := json.Marshal(report)
	require.NoError(t, err)

	var raw map[string]any

	err = json.Unmarshal(data, &raw)
	require.NoError(t, err)

	assert.Contains(t, raw, "isUp")
	assert.Contains(t, raw, "reports")
	assert.Contains(t, raw, "totalChecksRun")
	assert.Contains(t, raw, "timeSinceLastUpdate")
	assert.Contains(t, raw, "updUptime")
	assert.Contains(t, raw, "updVersion")
	assert.Contains(t, raw, "generatedAt")
}
