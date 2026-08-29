package cmd

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bulislaw/suuntool/internal/api"
)

func TestMCPCommand_DestructiveRequiresWrite(t *testing.T) {
	// Save/restore command state since rootCmd is package-global.
	origOut, origErr := rootCmd.OutOrStdout(), rootCmd.ErrOrStderr()
	defer func() {
		rootCmd.SetOut(origOut)
		rootCmd.SetErr(origErr)
		rootCmd.SetArgs(nil)
		flagMCPAllowWrite = false
		flagMCPAllowDestructive = false
	}()

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"mcp", "--allow-destructive"})

	err := rootCmd.ExecuteContext(context.Background())
	if err == nil {
		t.Fatalf("expected USAGE error, got nil. output: %s", buf.String())
	}
	var apiErr *api.Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *api.Error, got %T: %v", err, err)
	}
	if apiErr.Code != "USAGE" {
		t.Fatalf("expected Code=USAGE, got %s", apiErr.Code)
	}
	// Sanity: error message references the constraint.
	if !strings.Contains(apiErr.Message, "allow-destructive") {
		t.Fatalf("unexpected message: %s", apiErr.Message)
	}
}
