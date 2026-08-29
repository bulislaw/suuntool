package cmd

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tajchert/suuntool/internal/cache"
	"github.com/tajchert/suuntool/internal/session"
)

func TestWorkoutsSML_UsesPersistentCacheAndCountsHit(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("SUUNTOOL_SESSION_FILE", filepath.Join(tmp, "session.json"))
	require.NoError(t, session.Save(&session.Session{SessionKey: "SK", Username: "alice", UserKey: "uk1"}))

	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/workouts/wk1/sml" {
			http.NotFound(w, r)
			return
		}
		requests++
		_, _ = w.Write([]byte(`{"samples":[1,2,3]}`))
	}))
	defer srv.Close()
	t.Setenv("SUUNTOOL_BASE_URL", srv.URL+"/v1/")

	originalStore := openCacheStore
	openCacheStore = func(s *session.Session) (*cache.Store, error) { return cache.NewAt(tmp, s) }
	t.Cleanup(func() { openCacheStore = originalStore })

	originalOutput, originalFormat := flagOutput, flagFormat
	t.Cleanup(func() {
		flagOutput, flagFormat = originalOutput, originalFormat
		rootCmd.SetArgs(nil)
	})

	firstPath := filepath.Join(tmp, "first.json")
	rootCmd.SetArgs([]string{"workouts", "sml", "wk1", "-o", firstPath})
	require.NoError(t, rootCmd.Execute())
	require.Equal(t, 1, requests)

	secondPath := filepath.Join(tmp, "second.json")
	rootCmd.SetArgs([]string{"workouts", "sml", "wk1", "-o", secondPath})
	require.NoError(t, rootCmd.Execute())
	require.Equal(t, 1, requests)

	first, err := os.ReadFile(firstPath)
	require.NoError(t, err)
	second, err := os.ReadFile(secondPath)
	require.NoError(t, err)
	require.Equal(t, first, second)
}
