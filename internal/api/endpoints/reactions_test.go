package endpoints

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/bulislaw/suuntool/internal/api"
)

func TestAddReaction_PostsEmptyJSONBodyWithTotp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/workouts/reaction/wk1", r.URL.Path)
		require.Equal(t, "POST", r.Method)
		require.Contains(t, r.Header.Get("Content-Type"), "application/json")
		require.Equal(t, "123456", r.Header.Get("x-totp"))
		body, _ := io.ReadAll(r.Body)
		require.Empty(t, body)
		_, _ = w.Write([]byte(`{"error":null,"payload":"r_xyz"}`))
	}))
	defer srv.Close()
	c := api.NewClient(srv.URL+"/", "SK", time.Second)
	raw, err := AddReaction(context.Background(), c, "wk1", ReactionLike, map[string]string{"x-totp": "123456"})
	require.NoError(t, err)
	require.Contains(t, string(raw), "r_xyz")
}

func TestRemoveReaction_HitsCorrectPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/workouts/reaction/wk1", r.URL.Path)
		require.Equal(t, "DELETE", r.Method)
		_, _ = w.Write([]byte(`{"error":null,"payload":true}`))
	}))
	defer srv.Close()
	c := api.NewClient(srv.URL+"/", "SK", time.Second)
	require.NoError(t, RemoveReaction(context.Background(), c, "wk1"))
}
