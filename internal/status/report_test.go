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

func setupTestServer(t *testing.T, opts ...func(*StatServerConfig)) *StatHandler {
	t.Helper()

	status := NewStatus()
	status.SetRetention(1 * time.Hour)

	config := &StatServerConfig{
		Port:    8080,
		Reports: []time.Duration{time.Minute},
	}

	for _, opt := range opts {
		opt(config)
	}

	server := &StatServer{
		status: status,
		config: config,
	}

	return &StatHandler{statServer: server}
}

func TestStatHandlerInit(t *testing.T) {
	status := NewStatus()
	status.SetRetention(1 * time.Hour)

	config := &StatServerConfig{
		Port:    8080,
		Reports: []time.Duration{time.Minute},
	}

	server := &StatServer{
		status: status,
		config: config,
	}

	handler := &StatHandler{statServer: server}
	require.NotNil(t, handler)
	assert.Equal(t, server, handler.statServer)
}

func TestStatHandler_GenStatReport(t *testing.T) {
	handler := setupTestServer(t, func(c *StatServerConfig) {
		c.Reports = []time.Duration{
			time.Minute,
			5 * time.Minute,
		}
	})
	status := handler.statServer.status
	status.Update(true)

	report := handler.GenStatReport()
	require.NotNil(t, report)
	assert.True(t, report.Up)
	assert.Len(t, report.Stats, 2)
	assert.NotEmpty(t, report.Version)
	assert.False(t, report.Generated.IsZero())
}

func TestStatHandler_GenStatReport_WithChanges(t *testing.T) {
	handler := setupTestServer(t)
	status := handler.statServer.status
	status.Update(true)
	time.Sleep(10 * time.Millisecond)
	status.Update(false)
	time.Sleep(10 * time.Millisecond)
	status.Update(true)

	report := handler.GenStatReport()
	require.NotNil(t, report)
	require.NotNil(t, report.Loop)
	assert.True(t, report.Up)
	assert.GreaterOrEqual(t, report.Loop.TotalChecksRun, uint32(3))
}

func TestStatHandler_ServeHTTP(t *testing.T) {
	handler := setupTestServer(t)
	status := handler.statServer.status
	status.Update(true)

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
	handler := setupTestServer(t)
	status := handler.statServer.status
	status.Update(true)

	mux := http.NewServeMux()
	mux.Handle("GET "+StatRoute, handler)

	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, StatRoute, http.NoBody))

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestStatHandler_ServeHTTP_MethodHead(t *testing.T) {
	handler := setupTestServer(t)
	status := handler.statServer.status
	status.Update(true)

	req := httptest.NewRequest(http.MethodHead, StatRoute, http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestStatHandler_ServeHTTP_JSONFormat(t *testing.T) {
	handler := setupTestServer(t)
	status := handler.statServer.status
	status.Update(true)

	req := httptest.NewRequest(http.MethodGet, StatRoute, http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	body := rec.Body.String()
	assert.Contains(t, body, `"isUp": true`)
	assert.Contains(t, body, `"reports"`)
	assert.Contains(t, body, `"loop"`)
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
		Up:     true,
		Stats:  []ReportByPeriod{{Period: ReadableDuration(1 * time.Minute)}},
		Uptime: ReadableDuration(1 * time.Hour),
		Loop: &LoopStatus{
			Interval:        ReadableDuration(1 * time.Minute),
			TimeSinceUpdate: ReadableDuration(5 * time.Second),
			TotalChecksRun:  100,
		},
		Version:   "1.0.0",
		Generated: time.Now(),
	}

	data, err := json.MarshalIndent(report, "", JSONIndentSpaces)
	require.NoError(t, err)

	var raw map[string]any

	err = json.Unmarshal(data, &raw)
	require.NoError(t, err)

	assert.Equal(t, true, raw["isUp"])
	assert.Equal(t, "1.0.0", raw["updVersion"])

	loopRaw, ok := raw["loop"].(map[string]any)
	require.True(t, ok)
	assert.InEpsilon(t, float64(100), loopRaw["totalChecksRun"], 0.01)
}

func TestReport_JSONFieldNames(t *testing.T) {
	report := &Report{
		Up:        true,
		Stats:     []ReportByPeriod{},
		Uptime:    ReadableDuration(1 * time.Hour),
		Version:   "test-version",
		Generated: time.Now(),
		DownAction: &DownActionStatus{
			Iteration:     3,
			SleepTime:     ReadableDuration(15 * time.Second),
			BackoffCapped: true,
		},
		Loop: &LoopStatus{
			LastSuccess:     ReadableDuration(30 * time.Second),
			NextCheck:       ReadableDuration(1 * time.Minute),
			Interval:        ReadableDuration(1 * time.Minute),
			TimeSinceUpdate: ReadableDuration(10 * time.Second),
			TotalChecksRun:  42,
		},
	}

	data, err := json.Marshal(report)
	require.NoError(t, err)

	var raw map[string]any

	err = json.Unmarshal(data, &raw)
	require.NoError(t, err)

	assert.Contains(t, raw, "isUp")
	assert.Contains(t, raw, "reports")
	assert.Contains(t, raw, "updUptime")
	assert.Contains(t, raw, "updVersion")
	assert.Contains(t, raw, "generatedAt")
	assert.Contains(t, raw, "downAction")
	assert.Contains(t, raw, "loop")

	loopRaw, ok := raw["loop"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, loopRaw, "totalChecksRun")
	assert.Contains(t, loopRaw, "timeSinceLastUpdate")
}

func TestReport_SubStructFields(t *testing.T) {
	tests := []struct {
		name   string
		obj    any
		checks map[string]any
	}{
		{
			name: "DownActionStatus",
			obj: DownActionStatus{
				Iteration:     5,
				SleepTime:     ReadableDuration(22 * time.Second),
				BackoffCapped: true,
			},
			checks: map[string]any{
				"iteration":     float64(5),
				"sleepTime":     "22s",
				"backoffCapped": true,
			},
		},
		{
			name: "LoopStatus",
			obj: LoopStatus{
				LastSuccess:     ReadableDuration(45 * time.Second),
				NextCheck:       ReadableDuration(30 * time.Second),
				Interval:        ReadableDuration(30 * time.Second),
				TimeSinceUpdate: ReadableDuration(10 * time.Second),
				TotalChecksRun:  99,
			},
			checks: map[string]any{
				"lastSuccess":         "45s",
				"nextCheck":           "30s",
				"interval":            "30s",
				"timeSinceLastUpdate": "10s",
				"totalChecksRun":      float64(99),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.obj)
			require.NoError(t, err)

			var raw map[string]any

			err = json.Unmarshal(data, &raw)
			require.NoError(t, err)

			for key, want := range tt.checks {
				if f, ok := want.(float64); ok {
					assert.InDelta(t, f, raw[key], 0.01, "key %s", key)
				} else {
					assert.Equal(t, want, raw[key], "key %s", key)
				}
			}
		})
	}
}

func TestReport_DownActionOmittedWhenNil(t *testing.T) {
	report := &Report{
		Up:      true,
		Version: "test",
		Loop:    &LoopStatus{Interval: ReadableDuration(time.Minute)},
	}

	data, err := json.Marshal(report)
	require.NoError(t, err)

	var raw map[string]any

	err = json.Unmarshal(data, &raw)
	require.NoError(t, err)

	assert.NotContains(t, raw, "downAction")
	assert.Contains(t, raw, "loop")
}
