package check

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
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

	report := probe.Execute(context.Background(), testTimeout)
	require.NoError(t, report.error)
	assert.Equal(t, testOKStatus, report.response)
}

func TestHttpProbe_UserAgentHeader(t *testing.T) {
	var gotUA string

	probe := &HTTPProbe{
		URL: "http://example.com/ua",
		client: &http.Client{
			Transport: &updTransport{
				version: "dev",
				delegate: &fakeRoundTripper{
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
		},
	}

	report := probe.Execute(context.Background(), testTimeout)
	require.NoError(t, report.error)
	assert.Equal(t, "upd/dev", gotUA)
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

	report := probe.Execute(context.Background(), testTimeout)
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

	report := probe.Execute(context.Background(), testTimeout)
	checkTimeout(t, report, "context deadline exceeded")
}

func TestHTTPProbe_RoundTrip(t *testing.T) {
	var gotUA string

	ft := &fakeRoundTripper{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Status:     testOKStatus,
			Body:       io.NopCloser(strings.NewReader("ok")),
		},
		checkReq: func(req *http.Request) {
			gotUA = req.Header.Get("User-Agent")
		},
	}
	trans := &updTransport{version: "test", delegate: ft}
	req := httptest.NewRequest(http.MethodGet, testURL+"/rt", http.NoBody)
	resp, err := trans.RoundTrip(req)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	_ = resp.Body.Close()

	assert.Equal(t, "upd/test", gotUA)
}

func TestHTTPProbe_RoundTrip_NetworkFailure(t *testing.T) {
	trans := &updTransport{
		version:  "test",
		delegate: &fakeRoundTripper{err: errors.New("connection refused")},
	}
	req := httptest.NewRequest(http.MethodGet, testURL+"/test", http.NoBody)

	resp, err := trans.RoundTrip(req)

	if resp != nil {
		_ = resp.Body.Close()
	}

	assert.Error(t, err)
}

func TestHTTPProbe_ProbeWithTimeout(t *testing.T) {
	httpProbe := &HTTPProbe{URL: "://invalid", client: http.DefaultClient}
	report := httpProbe.Execute(context.Background(), time.Second)
	require.Error(t, report.error)
	assert.Contains(t, report.error.Error(), "error building request")
}

func TestHTTPProbe_Scheme(t *testing.T) {
	probe := NewHTTPProbe("http://example.com")
	assert.Equal(t, "http", probe.Scheme())
}
