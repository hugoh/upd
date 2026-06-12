package check

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeRoundTripper struct {
	resp     *http.Response
	err      error
	checkReq func(*http.Request)
}

func (f *fakeRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if f.checkReq != nil {
		f.checkReq(req)
	}

	return f.resp, f.err
}

func TestHttpProbe_Success(t *testing.T) {
	probe := &HTTPProbe{
		URL: testURL,
		client: &http.Client{
			Transport: &fakeRoundTripper{
				resp: &http.Response{
					StatusCode: http.StatusOK,
					Status:     testOKStatus,
					Body:       io.NopCloser(strings.NewReader("ok")),
				},
			},
		},
	}

	report := probe.Execute(t.Context(), testTimeout)
	require.NoError(t, report.error)
	assert.Equal(t, testOKStatus, report.response)
}

func TestHttpProbe_UserAgentHeader(t *testing.T) {
	var gotUA string

	probe := &HTTPProbe{
		URL: "http://example.com/ua",
		client: &http.Client{
			Transport: &fakeRoundTripper{
				resp: &http.Response{
					StatusCode: http.StatusOK,
					Status:     testOKStatus,
					Body:       io.NopCloser(strings.NewReader("pong")),
				},
				checkReq: func(req *http.Request) {
					gotUA = req.Header.Get("User-Agent")
				},
			},
		},
	}

	report := probe.Execute(t.Context(), testTimeout)
	require.NoError(t, report.error)
	assert.Equal(t, "upd/dev", gotUA)
}

func TestHttpProbe_DrainsBodyForConnectionReuse(t *testing.T) {
	body := strings.NewReader(strings.Repeat("x", 1024))
	probe := &HTTPProbe{
		URL: testURL,
		client: &http.Client{
			Transport: &fakeRoundTripper{
				resp: &http.Response{
					StatusCode: http.StatusOK,
					Status:     testOKStatus,
					Body:       io.NopCloser(body),
				},
			},
		},
	}

	report := probe.Execute(t.Context(), testTimeout)
	require.NoError(t, report.error)
	assert.Zero(t, body.Len(), "response body should be drained")
}

func TestHttpProbe_RequestFails(t *testing.T) {
	probe := &HTTPProbe{
		URL: testURL,
		client: &http.Client{
			Transport: &fakeRoundTripper{
				err: errors.New("connection refused"),
			},
		},
	}

	report := probe.Execute(t.Context(), testTimeout)
	err := checkError(t, report)
	assert.Contains(
		t,
		err.Error(),
		"connection refused",
	)
}

func TestHttpProbe_Timeout(t *testing.T) {
	probe := &HTTPProbe{
		URL: testURL,
		client: &http.Client{
			Transport: &fakeRoundTripper{
				err: context.DeadlineExceeded,
			},
		},
	}

	report := probe.Execute(t.Context(), testTimeout)
	checkTimeout(t, report, "context deadline exceeded")
}

func TestHTTPProbe_ProbeWithTimeout(t *testing.T) {
	httpProbe := &HTTPProbe{URL: "://invalid", client: http.DefaultClient}
	report := httpProbe.Execute(t.Context(), time.Second)
	require.Error(t, report.error)
	assert.Contains(t, report.error.Error(), "error building request")
}

func TestHTTPProbe_Target(t *testing.T) {
	probe := NewHTTPProbe("http://example.com/path")
	assert.Equal(t, "http://example.com/path", probe.Target())
}

func TestHTTPProbe_Scheme(t *testing.T) {
	probe := NewHTTPProbe("http://example.com")
	assert.Equal(t, "http", probe.Scheme())
}

func TestHTTPProbe_SchemeHTTPS(t *testing.T) {
	probe := NewHTTPProbe("https://example.com")
	assert.Equal(t, "https", probe.Scheme())
}
