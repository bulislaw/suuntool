package endpoints_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/bulislaw/suuntool/internal/api"
	"github.com/bulislaw/suuntool/internal/api/endpoints"
)

func TestListComments_DecodesAndPaths(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/workouts/comments/wk1", r.URL.Path)
		_, _ = w.Write([]byte(`{"error":null,"payload":[
			{"key":"c1","comment":"nice","username":"alice","timestamp":1700000000000}]}`))
	}))
	defer srv.Close()
	c := api.NewClient(srv.URL+"/", "SK", time.Second)
	got, err := endpoints.ListComments(context.Background(), c, "wk1")
	require.NoError(t, err)
	require.Len(t, got.Items, 1)
	require.Equal(t, "alice", got.Items[0].Username)
}

func TestPostComment_SendsTextBodyAndTotpHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/workouts/comment/wk1", r.URL.Path)
		require.Equal(t, "POST", r.Method)
		require.Contains(t, r.Header.Get("Content-Type"), "text/plain")
		require.Equal(t, "123456", r.Header.Get("x-totp"))
		body, _ := io.ReadAll(r.Body)
		require.Equal(t, "nice run", string(body))
		_, _ = w.Write([]byte(`{"error":null,"payload":{"key":"wk1","commentCount":1}}`))
	}))
	defer srv.Close()
	c := api.NewClient(srv.URL+"/", "SK", time.Second)
	raw, err := endpoints.PostComment(context.Background(), c, "wk1", "nice run", map[string]string{"x-totp": "123456"})
	require.NoError(t, err)
	require.Contains(t, string(raw), `"commentCount":1`)
}

func TestDeleteComment_HitsCorrectPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/workouts/comment/c1", r.URL.Path)
		require.Equal(t, "DELETE", r.Method)
		_, _ = w.Write([]byte(`{"error":null,"payload":true}`))
	}))
	defer srv.Close()
	c := api.NewClient(srv.URL+"/", "SK", time.Second)
	require.NoError(t, endpoints.DeleteComment(context.Background(), c, "c1"))
}
