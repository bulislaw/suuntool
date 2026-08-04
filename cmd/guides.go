package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/tajchert/suuntool/internal/api"
	"github.com/tajchert/suuntool/internal/api/endpoints"
	"github.com/tajchert/suuntool/internal/output"
)

var guidesCmd = &cobra.Command{
	Use:   "guides",
	Short: "SuuntoPlus Guide commands (list, download)",
	Long: `Structured-workout guide commands.

suuntool moves guide archives -- zip files containing manifest.json,
guide.json and icon.png -- as opaque bytes. It does not parse, build or
validate guide.json content; that's a workout-authoring concern for whatever
produces the archive.

Requires an active session (run 'suuntool login' first).`,
}

var guidesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all guides on the account",
	Long: `Fetch every guide on the account (GET suuntoplus/guides/items).

Unlike workouts, this endpoint takes no pagination -- there is no
--limit/--offset/--since here because the server accepts none.`,
	Example: `  suuntool guides list
  suuntool guides list --format tsv
  suuntool guides list --format json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := authedClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), pickTimeout())
		defer cancel()
		list, err := endpoints.ListGuides(ctx, c)
		if err != nil {
			return err
		}
		return emit(list)
	},
}

var guidesDownloadCmd = &cobra.Command{
	Use:   "download <id>",
	Short: "Download a guide's zip archive",
	Long: `Download the guide's zip archive from suuntoplus/guides/files/{id}.
Default writes to stdout; use -o to save to a file (recommended -- the
archive is binary).

Note: the server reconstitutes the archive from what it has stored rather
than echoing the original upload byte-for-byte -- expect an equivalent
archive, not identical bytes.`,
	Args:    cobra.ExactArgs(1),
	Example: `  suuntool guides download g1 -o g1.zip`,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := authedClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), pickTimeout())
		defer cancel()
		rc, err := endpoints.DownloadGuide(ctx, c, args[0])
		if err != nil {
			return err
		}
		defer rc.Close()

		// Raw passthrough — sanctioned bypass of emit() per CLAUDE.md / plan P4.
		// The archive is binary; Render doesn't apply and piping to stdout
		// unprompted is what -o exists to avoid.
		if flagOutput != "" {
			return output.WriteRaw(flagOutput, rc)
		}
		return output.WriteRawStdout(rc)
	},
}

var guidesUploadCmd = &cobra.Command{
	Use:   "upload <zip>",
	Short: "Upload a new guide archive",
	Long: `Upload a new guide archive (POST suuntoplus/guides/files). zip must be a
path to a zip file containing manifest.json, guide.json and icon.png;
suuntool sends it as-is and does not open or validate it.

A guide.json externalId that collides with an existing guide on the account
returns exit 5 (server) with the server's own "Conflict" description in the
error message -- there is no dedicated conflict exit code.`,
	Args: cobra.ExactArgs(1),
	Example: `  suuntool guides upload ./workout.zip
  suuntool guides upload ./workout.zip --format json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := authedClient()
		if err != nil {
			return err
		}
		f, err := os.Open(args[0])
		if err != nil {
			return &api.Error{Code: "USAGE", Message: err.Error(), Exit: ExitUsage}
		}
		defer f.Close()

		ctx, cancel := context.WithTimeout(cmd.Context(), pickTimeout())
		defer cancel()
		g, err := endpoints.CreateGuide(ctx, c, f)
		if err != nil {
			return err
		}
		if !flagQuiet {
			fmt.Fprintf(os.Stderr, "Created guide id=%s\n", g.ID)
		}
		return emit(g)
	},
}

var guidesUpdateCmd = &cobra.Command{
	Use:   "update <id> <zip>",
	Short: "Replace an existing guide's content",
	Long: `Replace an existing guide's content (PUT suuntoplus/guides/files/{id}).
Content only -- this does not change pinned status.`,
	Args:    cobra.ExactArgs(2),
	Example: `  suuntool guides update g1 ./workout-v2.zip`,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := authedClient()
		if err != nil {
			return err
		}
		f, err := os.Open(args[1])
		if err != nil {
			return &api.Error{Code: "USAGE", Message: err.Error(), Exit: ExitUsage}
		}
		defer f.Close()

		ctx, cancel := context.WithTimeout(cmd.Context(), pickTimeout())
		defer cancel()
		g, err := endpoints.UpdateGuide(ctx, c, args[0], f)
		if err != nil {
			return err
		}
		if !flagQuiet {
			fmt.Fprintf(os.Stderr, "Updated guide id=%s\n", g.ID)
		}
		return emit(g)
	},
}

var guidesPinCmd = &cobra.Command{
	Use:   "pin <id>",
	Short: "Pin a guide",
	Long: `Pin a guide (PATCH suuntoplus/guides/items/{id}). This is the only way to
change pinned status -- 'guides update' content-only PUT does not touch it.`,
	Args:    cobra.ExactArgs(1),
	Example: `  suuntool guides pin g1`,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := authedClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), pickTimeout())
		defer cancel()
		g, err := endpoints.SetGuidePinned(ctx, c, args[0], true)
		if err != nil {
			return err
		}
		return emit(g)
	},
}

var guidesUnpinCmd = &cobra.Command{
	Use:     "unpin <id>",
	Short:   "Unpin a guide",
	Long:    `Unpin a guide (PATCH suuntoplus/guides/items/{id}).`,
	Args:    cobra.ExactArgs(1),
	Example: `  suuntool guides unpin g1`,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := authedClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), pickTimeout())
		defer cancel()
		g, err := endpoints.SetGuidePinned(ctx, c, args[0], false)
		if err != nil {
			return err
		}
		return emit(g)
	},
}

var guidesPriorityCmd = &cobra.Command{
	Use:   "priority",
	Short: "Show the account's guide priority order",
	Long: `Fetch the account's guide priority order (GET suuntoplus/guides/priority).
Returns every guide on the account as {id}, ordered -- most recently pinned first.`,
	Example: `  suuntool guides priority`,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := authedClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), pickTimeout())
		defer cancel()
		p, err := endpoints.GuidePriority(ctx, c)
		if err != nil {
			return err
		}
		return emit(p)
	},
}

// guides delete <id>
var flagGuidesDeleteYes bool

var guidesDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Permanently delete a guide (destructive)",
	Long: `Permanently delete a guide. THIS CANNOT BE UNDONE.

By default, asks for interactive confirmation on a TTY. In non-TTY contexts
(scripts, agents, CI) you MUST pass --yes; otherwise the command exits with
code 2 (USAGE) without making any HTTP call.

Server endpoint: DELETE /v1/suuntoplus/guides/files/{id}.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		ok, err := confirm("Really delete guide "+id+"? This cannot be undone.", flagGuidesDeleteYes)
		if err != nil {
			return err
		}
		if !ok {
			if !flagQuiet {
				fmt.Fprintln(os.Stderr, "Aborted.")
			}
			return nil
		}
		c, _, err := authedClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), pickTimeout())
		defer cancel()
		if err := endpoints.DeleteGuide(ctx, c, id); err != nil {
			return err
		}
		if !flagQuiet {
			fmt.Fprintln(os.Stderr, "Deleted guide", id)
		}
		return nil
	},
	Example: `  suuntool guides delete g1          # interactive prompt on TTY
  suuntool guides delete g1 --yes    # non-interactive (scripts/agents)`,
}

func init() {
	guidesDeleteCmd.Flags().BoolVar(&flagGuidesDeleteYes, "yes", false, "Skip the confirmation prompt (required for non-TTY)")

	guidesCmd.AddCommand(guidesListCmd, guidesDownloadCmd, guidesUploadCmd, guidesUpdateCmd,
		guidesPinCmd, guidesUnpinCmd, guidesPriorityCmd, guidesDeleteCmd)
	rootCmd.AddCommand(guidesCmd)
}
