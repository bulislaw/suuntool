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

func TestFetchExtensions_PostsTypesArray(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "POST", r.Method)
		require.Equal(t, "/workout/extensions/wk1", r.URL.Path)
		require.Contains(t, r.Header.Get("Content-Type"), "application/json")
		body, _ := io.ReadAll(r.Body)
		var got []string
		require.NoError(t, json.Unmarshal(body, &got))
		require.Equal(t, []string{"FitnessExtension", "IntensityExtension"}, got)
		_, _ = w.Write([]byte(`{"error":null,"payload":{"FitnessExtension":{}}}`))
	}))
	defer srv.Close()
	c := api.NewClient(srv.URL+"/", "SK", time.Second)
	raw, err := FetchExtensions(context.Background(), c, "wk1", []string{"FitnessExtension", "IntensityExtension"})
	require.NoError(t, err)
	require.Contains(t, string(raw), "FitnessExtension")
}

func TestFetchExtensions_UsesDefaultsWhenEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var got []string
		require.NoError(t, json.Unmarshal(body, &got))
		require.ElementsMatch(t, DefaultExtensionTypes, got)
		_, _ = w.Write([]byte(`{"error":null,"payload":{}}`))
	}))
	defer srv.Close()
	c := api.NewClient(srv.URL+"/", "SK", time.Second)
	_, err := FetchExtensions(context.Background(), c, "wk1", nil)
	require.NoError(t, err)
}
