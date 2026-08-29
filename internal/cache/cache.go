// Package cache stores reusable, lossless download artifacts per Suunto account.
package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/bulislaw/suuntool/internal/metrics"
	"github.com/bulislaw/suuntool/internal/session"
)

// Kind is a supported immutable downloaded artifact.
type Kind string

const (
	WorkoutSML   Kind = "workout-sml"
	WorkoutFIT   Kind = "workout-fit"
	GuideArchive Kind = "guide-archive"
)

var allKinds = []Kind{WorkoutSML, WorkoutFIT, GuideArchive}

// Store is one account's cache. A disabled store behaves as an always-miss
// cache, which is safer than attempting to share unknown account identity.
type Store struct {
	root    string
	enabled bool
}

// New creates a store in the platform cache directory for sess.
func New(sess *session.Session) (*Store, error) {
	root, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	return NewAt(root, sess)
}

// NewAt is New with an explicit cache root, primarily for tests.
func NewAt(root string, sess *session.Session) (*Store, error) {
	identity := accountIdentity(sess)
	if identity == "" {
		return &Store{}, nil
	}
	digest := sha256.Sum256([]byte(identity))
	return &Store{root: filepath.Join(root, "suuntool", "v1", hex.EncodeToString(digest[:])), enabled: true}, nil
}

func accountIdentity(sess *session.Session) string {
	if sess == nil {
		return ""
	}
	if sess.UserKey != "" {
		return "userKey:" + sess.UserKey
	}
	if sess.Email != "" {
		return "email:" + strings.ToLower(sess.Email)
	}
	if sess.Username != "" {
		return "username:" + strings.ToLower(sess.Username)
	}
	return ""
}

// Open returns a cached artifact. A successful call is a cache hit.
func (s *Store) Open(ctx context.Context, kind Kind, id string) (io.ReadCloser, error) {
	if !s.enabled {
		return nil, os.ErrNotExist
	}
	f, err := os.Open(s.path(kind, id))
	if err != nil {
		return nil, err
	}
	metrics.FromContext(ctx).RecordCacheHit()
	return f, nil
}

// Put copies r into an atomically committed cache file. Callers should ignore
// an error here after a successful server read: caching is always best effort.
func (s *Store) Put(kind Kind, id string, r io.Reader) error {
	if !s.enabled {
		return nil
	}
	if err := os.MkdirAll(s.root, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(s.root, 0o700); err != nil {
		return err
	}
	dir := filepath.Dir(s.path(kind, id))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := io.Copy(tmp, r); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, s.path(kind, id))
}

// Remove deletes a single artifact. It is intentionally idempotent.
func (s *Store) Remove(kind Kind, id string) error {
	if !s.enabled {
		return nil
	}
	err := os.Remove(s.path(kind, id))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// Clear removes this account only. It is intentionally idempotent.
func (s *Store) Clear() error {
	if !s.enabled {
		return nil
	}
	err := os.RemoveAll(s.root)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// KindStatus is one artifact category's contribution to Status.
type KindStatus struct {
	Kind     Kind      `json:"kind"`
	Entries  int       `json:"entries"`
	Bytes    int64     `json:"bytes"`
	NewestAt time.Time `json:"newestAt,omitempty"`
}

// Status describes cached artifacts without opening them as cache hits.
type Status struct {
	Entries  int          `json:"entries"`
	Bytes    int64        `json:"bytes"`
	NewestAt time.Time    `json:"newestAt,omitempty"`
	Kinds    []KindStatus `json:"kinds"`
}

func (s *Store) Status() (Status, error) {
	status := Status{Kinds: make([]KindStatus, 0, len(allKinds))}
	if !s.enabled {
		for _, kind := range allKinds {
			status.Kinds = append(status.Kinds, KindStatus{Kind: kind})
		}
		return status, nil
	}
	for _, kind := range allKinds {
		ks := KindStatus{Kind: kind}
		dir := filepath.Join(s.root, string(kind))
		entries, err := os.ReadDir(dir)
		if errors.Is(err, os.ErrNotExist) {
			status.Kinds = append(status.Kinds, ks)
			continue
		}
		if err != nil {
			return Status{}, err
		}
		for _, entry := range entries {
			if entry.IsDir() || strings.HasPrefix(entry.Name(), ".tmp-") {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				continue
			}
			ks.Entries++
			ks.Bytes += info.Size()
			if info.ModTime().After(ks.NewestAt) {
				ks.NewestAt = info.ModTime()
			}
		}
		status.Entries += ks.Entries
		status.Bytes += ks.Bytes
		if ks.NewestAt.After(status.NewestAt) {
			status.NewestAt = ks.NewestAt
		}
		status.Kinds = append(status.Kinds, ks)
	}
	sort.Slice(status.Kinds, func(i, j int) bool { return status.Kinds[i].Kind < status.Kinds[j].Kind })
	return status, nil
}

func (s *Store) path(kind Kind, id string) string {
	digest := sha256.Sum256([]byte(string(kind) + "\x00" + id))
	return filepath.Join(s.root, string(kind), hex.EncodeToString(digest[:]))
}

// Cached returns a cached reader or fetches and persists an artifact. Cache
// writes happen after the full download is read, so a failed cache write never
// changes the bytes returned by the caller.
func Cached(ctx context.Context, s *Store, kind Kind, id string, fetch func() (io.ReadCloser, error)) (io.ReadCloser, error) {
	if s != nil {
		if cached, err := s.Open(ctx, kind, id); err == nil {
			return cached, nil
		}
	}
	source, err := fetch()
	if err != nil {
		return nil, err
	}
	if s == nil || !s.enabled {
		return source, nil
	}
	return &capturingReader{ReadCloser: source, store: s, kind: kind, id: id}, nil
}

type capturingReader struct {
	io.ReadCloser
	store     *Store
	kind      Kind
	id        string
	tmp       *os.File
	tmpName   string
	failed    bool
	committed bool
}

func (r *capturingReader) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)
	if n > 0 {
		r.capture(p[:n])
	}
	if err == io.EOF {
		r.commit()
	}
	return n, err
}

func (r *capturingReader) capture(p []byte) {
	if r.failed {
		return
	}
	if r.tmp == nil {
		dir := filepath.Dir(r.store.path(r.kind, r.id))
		if err := os.MkdirAll(dir, 0o700); err != nil {
			r.failed = true
			return
		}
		if err := os.Chmod(r.store.root, 0o700); err != nil {
			r.failed = true
			return
		}
		if err := os.Chmod(dir, 0o700); err != nil {
			r.failed = true
			return
		}
		tmp, err := os.CreateTemp(dir, ".tmp-*")
		if err != nil {
			r.failed = true
			return
		}
		if err := tmp.Chmod(0o600); err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmp.Name())
			r.failed = true
			return
		}
		r.tmp, r.tmpName = tmp, tmp.Name()
	}
	if _, err := r.tmp.Write(p); err != nil {
		r.failed = true
		_ = r.tmp.Close()
		_ = os.Remove(r.tmpName)
		r.tmp = nil
	}
}

func (r *capturingReader) commit() {
	if r.failed || r.tmp == nil || r.committed {
		return
	}
	if err := r.tmp.Close(); err == nil {
		if err := os.Rename(r.tmpName, r.store.path(r.kind, r.id)); err == nil {
			r.committed = true
		}
	}
	if !r.committed {
		_ = os.Remove(r.tmpName)
	}
	r.tmp = nil
}

func (r *capturingReader) Close() error {
	if r.tmp != nil && !r.committed {
		_ = r.tmp.Close()
		_ = os.Remove(r.tmpName)
		r.tmp = nil
	}
	return r.ReadCloser.Close()
}

func (s Status) String() string {
	return fmt.Sprintf("cache entries: %d\ncache size:    %d bytes", s.Entries, s.Bytes)
}
