package endpoints

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/bulislaw/suuntool/internal/api"
)

// gzipNDJSON compresses the given NDJSON lines into a gzip stream.
func gzipNDJSON(t *testing.T, lines []string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	for _, l := range lines {
		_, err := w.Write([]byte(l + "\n"))
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())
	return buf.Bytes()
}

func TestFetchWellness_GzippedNDJSONRoundTrip(t *testing.T) {
	type entry struct {
		Timestamp string         `json:"timestamp"`
		EntryData map[string]int `json:"entryData"`
	}
	input := []entry{
		{Timestamp: "2026-05-11T00:00:00Z", EntryData: map[string]int{"k": 1}},
		{Timestamp: "2026-05-11T01:00:00Z", EntryData: map[string]int{"k": 2}},
		{Timestamp: "2026-05-11T02:00:00Z", EntryData: map[string]int{"k": 3}},
	}

	var lines []string
	for _, e := range input {
		b, err := json.Marshal(e)
		require.NoError(t, err)
		lines = append(lines, string(b))
	}

	// Gzip payload WITHOUT Content-Encoding header — mirrors Suunto behavior.
	compressed := gzipNDJSON(t, lines)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// No Content-Encoding header — DoStream uses magic-bytes detection.
		_, _ = w.Write(compressed)
	}))
	defer srv.Close()

	c := api.NewClient(srv.URL+"/", "SK123", time.Second)
	rc, err := FetchWellness(context.Background(), c, StreamSleep, 12345)
	require.NoError(t, err)
	defer rc.Close()

	all, err := io.ReadAll(rc)
	require.NoError(t, err)

	scanner := bufio.NewScanner(bytes.NewReader(all))
	var got []entry
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var e entry
		require.NoError(t, json.Unmarshal([]byte(line), &e))
		got = append(got, e)
	}
	require.NoError(t, scanner.Err())
	require.Equal(t, input, got)
}

func TestFetchWellness_PathAndSinceParam(t *testing.T) {
	var capturedPath string
	var capturedQuery string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"timestamp":"2026-05-11T00:00:00Z","entryData":{"k":1}}` + "\n"))
	}))
	defer srv.Close()

	c := api.NewClient(srv.URL+"/", "SK123", time.Second)
	rc, err := FetchWellness(context.Background(), c, StreamSleep, 12345)
	require.NoError(t, err)
	if rc != nil {
		rc.Close()
	}

	require.Equal(t, "/v1/sleep/export", capturedPath)
	require.Equal(t, "since=12345", capturedQuery)
}
