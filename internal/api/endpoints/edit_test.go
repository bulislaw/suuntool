package endpoints

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/bulislaw/suuntool/internal/api"
)

func TestEditWorkout_PutsPartialJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "PUT", r.Method)
		require.Equal(t, "/workouts/wk1/attributes", r.URL.Path)
		require.Contains(t, r.Header.Get("Content-Type"), "application/json")
		body, _ := io.ReadAll(r.Body)
		var got map[string]any
		require.NoError(t, json.Unmarshal(body, &got))
		require.Equal(t, 100.0, got["totalAscent"])
		_, _ = w.Write([]byte(`{"error":null,"payload":{"key":"wk1","totalAscent":100}}`))
	}))
	defer srv.Close()
	c := api.NewClient(srv.URL+"/", "SK", time.Second)
	raw, err := EditWorkout(context.Background(), c, "wk1", map[string]any{"totalAscent": 100.0})
	require.NoError(t, err)
	require.Contains(t, string(raw), `"totalAscent":100`)
}

func TestBatchUpdate_PostsJSONArray(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "POST", r.Method)
		require.Equal(t, "/workouts/batchUpdate", r.URL.Path)
		body, _ := io.ReadAll(r.Body)
		var got []map[string]any
		require.NoError(t, json.Unmarshal(body, &got))
		require.Len(t, got, 2)
		require.Equal(t, "wk1", got[0]["key"])
		require.Equal(t, "wk2", got[1]["key"])
		_, _ = w.Write([]byte(`{"error":null,"payload":true}`))
	}))
	defer srv.Close()
	c := api.NewClient(srv.URL+"/", "SK", time.Second)
	raw, err := BatchUpdate(context.Background(), c, []map[string]any{
		{"key": "wk1", "totalAscent": 50.0},
		{"key": "wk2", "name": "Morning run"},
	})
	require.NoError(t, err)
	require.Contains(t, string(raw), "true")
}
