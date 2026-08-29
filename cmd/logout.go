package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/tajchert/suuntool/internal/api/endpoints"
	"github.com/tajchert/suuntool/internal/session"
)

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Invalidate the current session and remove it from disk",
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := session.Load()
		if err != nil {
			if errors.Is(err, session.ErrNoSession) {
				fmt.Fprintln(os.Stderr, "Already logged out.")
				return nil
			}
			return err
		}

		ctx, cancel := context.WithTimeout(cmd.Context(), pickTimeout())
		defer cancel()

		if err := endpoints.Logout(ctx, baseURL(), s.SessionKey); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: server-side logout failed: %v\n", err)
		}

		if err := session.Clear(); err != nil {
			return err
		}

		if !flagQuiet {
			fmt.Fprintln(os.Stderr, "Logged out.")
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(logoutCmd)
}
