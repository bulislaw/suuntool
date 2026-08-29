// Package session manages the persisted Suunto auth session on disk.
package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/bulislaw/suuntool/internal/auth"
)

var ErrNoSession = errors.New("no saved session")

// Session is what we persist after a successful login.
type Session struct {
	SessionKey string `json:"sessionkey"`
	Username   string `json:"username"`
	Email      string `json:"email,omitempty"`
	UserKey    string `json:"userKey,omitempty"`
	Country    string `json:"country,omitempty"`
	OffsetMS   int64  `json:"server_time_offset_ms"`
	SavedAt    string `json:"saved_at"`
}

// Path returns the session file path, honoring SUUNTOOL_SESSION_FILE.
func Path() string {
	if p := os.Getenv("SUUNTOOL_SESSION_FILE"); p != "" {
		return p
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "suuntool", "session.json")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "suuntool", "session.json")
}

func Save(s *Session) error {
	s.SavedAt = time.Now().UTC().Format(time.RFC3339)
	p := Path()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return fmt.Errorf("session: mkdir: %w", err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(p, data, 0o600); err != nil {
		return fmt.Errorf("session: write: %w", err)
	}
	return nil
}

func Load() (*Session, error) {
	p := Path()
	data, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNoSession
		}
		return nil, err
	}
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("session: parse: %w", err)
	}
	return &s, nil
}

func Clear() error {
	err := os.Remove(Path())
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// TOTPHeaders returns the x-totp request header required by Suunto write
// endpoints (reactions, comments, settings-safe, email/phone change). The
// TOTP rotates every 30s; call this fresh per request.
func TOTPHeaders(s *Session) map[string]string {
	return map[string]string{
		"x-totp": auth.GenerateTOTP(s.Email, s.OffsetMS),
	}
}
