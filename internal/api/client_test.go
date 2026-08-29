package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/bulislaw/suuntool/internal/metrics"
)

func TestClient_InjectsSessionAndUserAgent(t *testing.T) {
	var gotAuth, gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("STTAuthorization")
		gotUA = r.Header.Get("User-Agent")
		_, _ = w.Write([]byte(`{"error":null,"payload":{"ok":true}}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL+"/", "SK123", time.Second)
	body, err := c.Do(context.Background(), "GET", "ping", nil, nil)
	require.NoError(t, err)
	require.Contains(t, string(body), `"ok":true`)
	require.Equal(t, "SK123", gotAuth)
	require.Contains(t, gotUA, "com.stt.android.suunto/")
}

func TestClient_RecordsRequestAttempt(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	counts := metrics.New()
	ctx := metrics.WithCounters(context.Background(), counts)
	c := NewClient(srv.URL+"/", "SK123", time.Second)
	_, err := c.Do(ctx, "GET", "ping", nil, nil)
	require.Error(t, err)
	require.Equal(t, int64(1), counts.Snapshot().ServerRequests)
}

func TestClient_MapsStatusCodesToTypedErrors(t *testing.T) {
	for _, tc := range []struct {
		status   int
		wantCode string
		wantExit int
	}{
		{401, "AUTH_EXPIRED", 4},
		{403, "FORBIDDEN", 7},
		{404, "NOT_FOUND", 6},
		{500, "SERVER", 5},
	} {
		t.Run(tc.wantCode, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
			}))
			defer srv.Close()
			c := NewClient(srv.URL+"/", "SK", time.Second)
			_, err := c.Do(context.Background(), "GET", "x", nil, nil)
			require.Error(t, err)
			e, ok := err.(*Error)
			require.True(t, ok)
			require.Equal(t, tc.wantCode, e.Code)
			require.Equal(t, tc.wantExit, e.Exit)
		})
	}
}

func TestDecodeAsko_SurfacesServerError(t *testing.T) {
	_, err := DecodeAsko[map[string]any]([]byte(`{"error":{"code":42,"description":"nope"},"payload":null}`))
	require.Error(t, err)
	e, ok := err.(*Error)
	require.True(t, ok)
	require.Equal(t, "SERVER", e.Code)
	require.Contains(t, e.Message, "nope")
}
