package mcp

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"

	"github.com/bulislaw/suuntool/internal/session"
)

type bearerTransport struct{ token string }

func (b bearerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(r)
}

func TestHTTPAuthAndHealth(t *testing.T) {
	h := newHTTPHandler(newServer(Opts{}), "secret")
	for _, tc := range []struct {
		path, token string
		status      int
	}{
		{"/healthz", "", 200}, {"/mcp", "", 401}, {"/mcp", "wrong", 401},
	} {
		r := httptest.NewRequest(http.MethodGet, tc.path, nil)
		if tc.token != "" {
			r.Header.Set("Authorization", "Bearer "+tc.token)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		require.Equal(t, tc.status, w.Code)
	}
	r := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	r.Header.Set("Authorization", "Bearer secret")
	r.Header.Set("Origin", "https://untrusted.example")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	require.Equal(t, http.StatusForbidden, w.Code)
	require.ErrorContains(t, Run(context.Background(), Opts{Listen: ":0"}), "SUUNTOOL_MCP_TOKEN")
}

// Exercise the production session/cache wiring over actual HTTP, including a
// new MCP client and a reconstructed server using the same persistent paths.
func TestHTTPCacheAcrossCallsClientsAndRestart(t *testing.T) {
	// macOS derives UserCacheDir from HOME rather than XDG_CACHE_HOME.
	t.Setenv("HOME", t.TempDir())
	t.Setenv("SUUNTOOL_SESSION_FILE", filepath.Join(t.TempDir(), "session.json"))
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	require.NoError(t, session.Save(authSession()))
	var requests atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/workouts/w1/sml", r.URL.Path)
		requests.Add(1)
		_, _ = io.WriteString(w, `{"Data":{"Samples":[{"TimeISO8601":"2026-01-01T00:00:00Z","Sample":{"HR":120}}]}}`)
	}))
	defer upstream.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for restart := 0; restart < 2; restart++ {
		srv := httptest.NewServer(newHTTPHandler(newServer(Opts{BaseURL: upstream.URL + "/v1/"}), "secret"))
		for client := 0; client < 2; client++ {
			c := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test", Version: "0"}, nil)
			cs, err := c.Connect(ctx, &sdkmcp.StreamableClientTransport{
				Endpoint: srv.URL + "/mcp", HTTPClient: &http.Client{Transport: bearerTransport{"secret"}},
			}, nil)
			require.NoError(t, err)
			for call := 0; call < 2; call++ {
				mustOK(t, callTool(t, cs, "workouts_sml", map[string]any{"key": "w1", "streams": []string{"HR"}}))
			}
			require.NoError(t, cs.Close())
		}
		srv.Close()
	}
	require.EqualValues(t, 1, requests.Load())
}

func TestHTTPShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- serveHTTP(ctx, "127.0.0.1:0", http.NotFoundHandler()) }()
	cancel()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("HTTP server did not stop on cancellation")
	}
}

func TestHTTPShutdownWaitsForActiveRequest(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{})
	release := make(chan struct{})
	releaseHandler := sync.OnceFunc(func() { close(release) })
	defer releaseHandler()
	done := make(chan error, 1)
	go func() {
		done <- serveHTTPListener(ctx, listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			close(started)
			<-release
			_, _ = io.WriteString(w, "completed")
		}))
	}()
	type response struct {
		body string
		err  error
	}
	responseDone := make(chan response, 1)
	go func() {
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get("http://" + listener.Addr().String())
		if err != nil {
			responseDone <- response{err: err}
			return
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		responseDone <- response{body: string(body), err: err}
	}()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("HTTP request did not start")
	}
	cancel()
	select {
	case err := <-done:
		t.Fatalf("HTTP server returned before the active request completed: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	releaseHandler()
	select {
	case result := <-responseDone:
		require.NoError(t, result.err)
		require.Equal(t, "completed", result.body)
	case <-time.After(3 * time.Second):
		t.Fatal("HTTP response did not complete")
	}
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("HTTP server did not stop after the active request completed")
	}
}
