package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bulislaw/suuntool/internal/session"
)

func TestTotpHeaders_ProducesSixDigitToken(t *testing.T) {
	s := &session.Session{Email: "alice@example.com", OffsetMS: 0}
	got := session.TOTPHeaders(s)
	tok, ok := got["x-totp"]
	require.True(t, ok, "x-totp header missing")
	require.Len(t, tok, 6, "TOTP should be 6 digits")
	for _, r := range tok {
		require.True(t, r >= '0' && r <= '9', "TOTP must be all digits, got %q", tok)
	}
}

func TestMergeHeaders_RightWins(t *testing.T) {
	a := map[string]string{"A": "1", "B": "2"}
	b := map[string]string{"B": "3", "C": "4"}
	got := mergeHeaders(a, b)
	require.Equal(t, "1", got["A"])
	require.Equal(t, "3", got["B"])
	require.Equal(t, "4", got["C"])
	require.Equal(t, "1", a["A"], "input must not be mutated")
}
