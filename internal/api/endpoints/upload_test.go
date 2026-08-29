package endpoints

import (
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/bulislaw/suuntool/internal/api"
)

func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(p, []byte(content), 0o600))
	return p
}

func TestUploadWorkout_PostsMultipartFilePart(t *testing.T) {
	smlPath := writeTemp(t, "wk.sml", `<sml>fake</sml>`)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "POST", r.Method)
		require.Equal(t, "/workout", r.URL.Path)
		mediatype, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		require.NoError(t, err)
		require.Equal(t, "multipart/form-data", mediatype)

		mr := multipart.NewReader(r.Body, params["boundary"])
		part, err := mr.NextPart()
		require.NoError(t, err)
		require.Equal(t, "filePart", part.FormName())
		got, _ := io.ReadAll(part)
		require.Equal(t, "<sml>fake</sml>", string(got))

		_, _ = w.Write([]byte(`{"error":null,"payload":{"key":"wk_new","username":"alice","activityId":1,"startTime":1700000000000,"totalDistance":5000.0,"totalTime":1800.0}}`))
	}))
	defer srv.Close()
	c := api.NewClient(srv.URL+"/", "SK", time.Second)
	got, err := UploadWorkout(context.Background(), c, smlPath, "")
	require.NoError(t, err)
	require.Equal(t, "wk_new", got.Key)
	require.Equal(t, "alice", got.Username)
}

func TestUploadWorkout_WithExtensions(t *testing.T) {
	smlPath := writeTemp(t, "wk.sml", "sml")
	extPath := writeTemp(t, "ext.json", `{"FitnessExtension":{}}`)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		require.NoError(t, err)
		mr := multipart.NewReader(r.Body, params["boundary"])
		names := []string{}
		for {
			p, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
			names = append(names, p.FormName())
			_, _ = io.Copy(io.Discard, p)
		}
		require.Equal(t, []string{"filePart", "workoutExtensionsPart"}, names)
		_, _ = w.Write([]byte(`{"error":null,"payload":{"key":"wk_new","username":"alice"}}`))
	}))
	defer srv.Close()
	c := api.NewClient(srv.URL+"/", "SK", time.Second)
	got, err := UploadWorkout(context.Background(), c, smlPath, extPath)
	require.NoError(t, err)
	require.Equal(t, "wk_new", got.Key)
}

func TestUploadWorkout_RejectsMissingSML(t *testing.T) {
	c := api.NewClient("https://example.invalid/", "SK", time.Second)
	_, err := UploadWorkout(context.Background(), c, "/nope/missing.sml", "")
	require.Error(t, err)
	e, ok := err.(*api.Error)
	require.True(t, ok)
	require.Equal(t, "USAGE", e.Code)
}
