package httpclient

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestClientRetryReplaysJSONRequestBody(t *testing.T) {
	var attempts atomic.Int32
	var bodies []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(body))

		if attempts.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL, WithHTTPClient(server.Client()), WithRetry(testRetryOptions()))
	err := client.Post(context.Background(), "/", map[string]string{"name": "launch"}, nil)

	require.NoError(t, err)
	require.Equal(t, int32(2), attempts.Load())
	require.Equal(t, []string{`{"name":"launch"}`, `{"name":"launch"}`}, bodies)
}

func TestClientRetryDoesNotReplayUnrepeatableBody(t *testing.T) {
	var attempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		attempts.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := NewClient(server.URL, WithHTTPClient(server.Client()), WithRetry(testRetryOptions()))
	response, err := client.PostReq("/").WithRawBody(&oneShotReader{Reader: strings.NewReader("payload")}, "text/plain").DoResponse(context.Background())

	require.NoError(t, err)
	require.Equal(t, http.StatusServiceUnavailable, response.StatusCode)
	require.Equal(t, int32(1), attempts.Load())
}

func TestWithRetryDoesNotMutateSharedOptions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL, WithHTTPClient(server.Client()), WithRetry(RetryOptions{}))
	err := client.Get(context.Background(), "/", nil)

	require.NoError(t, err)
	require.Equal(t, RetryOptions{}, *client.retryOpts)
}

func testRetryOptions() RetryOptions {
	return RetryOptions{
		MaxAttempts:  2,
		InitialDelay: time.Nanosecond,
		MaxDelay:     time.Nanosecond,
		Multiplier:   1,
	}
}

type oneShotReader struct{ io.Reader }
