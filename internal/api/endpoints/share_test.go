package endpoints

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/bulislaw/suuntool/internal/api"
)

func TestShareWorkout_PutsWithBrandHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "PUT", r.Method)
		require.Equal(t, "/workouts/alice/wk1/share/gpx-track", r.URL.Path)
		require.Equal(t, "suuntoapp", r.Header.Get("Brand"))
		_, _ = w.Write([]byte(`{"error":null,"payload":"https://maps.example/share/abc"}`))
	}))
	defer srv.Close()
	c := api.NewClient(srv.URL+"/", "SK", time.Second)
	url, err := ShareWorkout(context.Background(), c, "alice", "wk1", ShareGPXTrack)
	require.NoError(t, err)
	require.Equal(t, "https://maps.example/share/abc", url)
}
