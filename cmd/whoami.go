package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/tajchert/suuntool/internal/api/endpoints"
)

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show the profile of the currently authenticated user",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := authedClient()
		if err != nil {
			return err
		}

		ctx, cancel := context.WithTimeout(cmd.Context(), pickTimeout())
		defer cancel()

		u, err := endpoints.Whoami(ctx, c)
		if err != nil {
			return err
		}

		return emit(u)
	},
}

func init() {
	rootCmd.AddCommand(whoamiCmd)
}
