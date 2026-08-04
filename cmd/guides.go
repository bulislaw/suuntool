package cmd

import (
	"context"

	"github.com/spf13/cobra"

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
	Args: cobra.ExactArgs(1),
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

func init() {
	guidesCmd.AddCommand(guidesListCmd, guidesDownloadCmd)
	rootCmd.AddCommand(guidesCmd)
}
