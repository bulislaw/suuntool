package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bulislaw/suuntool/internal/api/endpoints"
)

var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Profile information (settings, follow stats, user lookup)",
	Long:  `Read-only profile commands. Requires an active session (run 'suuntool login' first).`,
}

var profileSettingsCmd = &cobra.Command{
	Use:   "settings",
	Short: "Show /v1/user/settings (full DTO)",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := authedClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), pickTimeout())
		defer cancel()
		raw, err := endpoints.Settings(ctx, c)
		if err != nil {
			return err
		}
		return emit(raw)
	},
}

var profileFollowCmd = &cobra.Command{
	Use:   "follow",
	Short: "Show follower / following counts",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := authedClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), pickTimeout())
		defer cancel()
		f, err := endpoints.Follow(ctx, c)
		if err != nil {
			return err
		}
		return emit(f)
	},
}

var profileUserCmd = &cobra.Command{
	Use:   "user <username>",
	Short: "Look up any user by username",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := authedClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), pickTimeout())
		defer cancel()
		u, err := endpoints.UserByName(ctx, c, args[0])
		if err != nil {
			return err
		}
		return emit(u)
	},
	Example: `  suuntool profile user michal
  suuntool profile user michal --format json
  suuntool profile user michal -o user.json`,
}

func init() {
	profileCmd.AddCommand(profileSettingsCmd, profileFollowCmd, profileUserCmd)
	rootCmd.AddCommand(profileCmd)
	_ = fmt.Sprintf // silence unused import if cobra prunes
}
