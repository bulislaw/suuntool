package cache

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bulislaw/suuntool/internal/metrics"
	"github.com/bulislaw/suuntool/internal/session"
)

func TestCached_RoundTripIsolationAndStatus(t *testing.T) {
	root := t.TempDir()
	a, err := NewAt(root, &session.Session{UserKey: "account-a"})
	require.NoError(t, err)
	b, err := NewAt(root, &session.Session{UserKey: "account-b"})
	require.NoError(t, err)

	ctx := metrics.WithCounters(context.Background(), metrics.New())
	fetches := 0
	read, err := Cached(ctx, a, WorkoutFIT, "wk-secret", func() (io.ReadCloser, error) {
		fetches++
		return io.NopCloser(bytes.NewReader([]byte("fit-bytes"))), nil
	})
	require.NoError(t, err)
	got, err := io.ReadAll(read)
	require.NoError(t, err)
	require.NoError(t, read.Close())
	require.Equal(t, []byte("fit-bytes"), got)
	require.Equal(t, 1, fetches)

	read, err = Cached(ctx, a, WorkoutFIT, "wk-secret", func() (io.ReadCloser, error) {
		fetches++
		return nil, os.ErrNotExist
	})
	require.NoError(t, err)
	got, err = io.ReadAll(read)
	require.NoError(t, err)
	require.NoError(t, read.Close())
	require.Equal(t, []byte("fit-bytes"), got)
	require.Equal(t, 1, fetches)
	require.Equal(t, int64(1), metrics.FromContext(ctx).Snapshot().CacheHits)

	_, err = b.Open(ctx, WorkoutFIT, "wk-secret")
	require.ErrorIs(t, err, os.ErrNotExist)

	status, err := a.Status()
	require.NoError(t, err)
	require.Equal(t, 1, status.Entries)
	require.Equal(t, int64(len("fit-bytes")), status.Bytes)
	require.NotEmpty(t, a.root)
	require.NotContains(t, a.root, "account-a")
	require.NotContains(t, a.path(WorkoutFIT, "wk-secret"), "wk-secret")

	info, err := os.Stat(a.path(WorkoutFIT, "wk-secret"))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	rootInfo, err := os.Stat(a.root)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o700), rootInfo.Mode().Perm())
}

func TestCached_DoesNotCommitPartialReadAndClearIsIdempotent(t *testing.T) {
	store, err := NewAt(t.TempDir(), &session.Session{UserKey: "account-a"})
	require.NoError(t, err)

	read, err := Cached(context.Background(), store, WorkoutSML, "wk1", func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader([]byte("partial"))), nil
	})
	require.NoError(t, err)
	buf := make([]byte, 2)
	_, err = read.Read(buf)
	require.NoError(t, err)
	require.NoError(t, read.Close())
	_, err = store.Open(context.Background(), WorkoutSML, "wk1")
	require.ErrorIs(t, err, os.ErrNotExist)

	require.NoError(t, store.Clear())
	require.NoError(t, store.Clear())
	status, err := store.Status()
	require.NoError(t, err)
	require.Zero(t, status.Entries)
}
