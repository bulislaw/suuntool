package cmd

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/bulislaw/suuntool/internal/api"
	"github.com/bulislaw/suuntool/internal/api/endpoints"
	"github.com/bulislaw/suuntool/internal/output"
	"github.com/bulislaw/suuntool/internal/session"
)

var (
	flagWellnessSince  string
	flagWellnessOutDir string
)

var wellnessCmd = &cobra.Command{
	Use:   "wellness",
	Short: "24/7 wellness exports (sleep, activity, recovery, sleepstages)",
	Long: `Stream 24/7 wellness data from the timeline service at 247.sports-tracker.com.

Each subcommand emits gzipped NDJSON which the client decodes on the fly to plain NDJSON
(one JSON object per line). Use --since to limit the cursor (accepts unix ms, RFC3339,
YYYY-MM-DD, a duration like 7d/2h, or a keyword like today/yesterday/last-week; empty = all history).

Unit quirks (pass-through, NOT normalized — see handoff §5):
  - hrAvg, hrMin are in Hz (beats per second). Multiply by 60 for BPM.
  - durations are in seconds (float).
  - quality, maxSpo2, balance are 0..1 fractions. Multiply by 100 for percent.`,
}

func newWellnessStreamCmd(stream endpoints.WellnessStream) *cobra.Command {
	cmd := &cobra.Command{
		Use:   string(stream),
		Short: "Export " + string(stream) + " entries as NDJSON",
		Example: "  suuntool wellness " + string(stream) + " > " + string(stream) + ".ndjson\n" +
			"  suuntool wellness " + string(stream) + " --since 7d -o " + string(stream) + ".ndjson\n" +
			"  suuntool wellness " + string(stream) + " --out ./export",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := session.Load()
			if err != nil {
				return &api.Error{Code: "AUTH_EXPIRED", Message: "no saved session", Hint: "Run: suuntool login", Exit: 4}
			}
			sinceMS, err := parseSince(flagWellnessSince)
			if err != nil {
				return &api.Error{Code: "USAGE", Message: "--since: " + err.Error(), Exit: 2}
			}
			c := api.NewTimelineClient(s.SessionKey, pickTimeout())
			ctx, cancel := context.WithTimeout(cmd.Context(), pickTimeout())
			defer cancel()
			body, err := endpoints.FetchWellness(ctx, c, stream, sinceMS)
			if err != nil {
				return err
			}
			defer body.Close()
			// Sleep gets a pretty-table mode on TTY / --format=pretty.
			// Other streams remain raw NDJSON for now (different shapes; row-wise
			// rendering not yet meaningful).
			if stream == endpoints.StreamSleep && shouldPrettySleep() {
				return renderSleepPretty(os.Stdout, body)
			}
			return writeWellness(string(stream), body)
		},
	}
	return cmd
}

// shouldPrettySleep returns true when the sleep command should emit a
// human-readable table instead of raw NDJSON. Pretty wins when:
//   - --format=pretty is explicit, OR
//   - --format=auto (default) and stdout is a TTY AND no file sink is set.
//
// Any file sink (--out, -o) forces raw NDJSON to preserve fidelity.
func shouldPrettySleep() bool {
	if flagWellnessOutDir != "" || flagOutput != "" {
		return false
	}
	switch flagFormat {
	case "pretty":
		return true
	case "json":
		return false
	default: // "auto" or empty
		return output.IsStdoutTTY()
	}
}

// writeWellness picks the right sink for raw NDJSON output:
//   - --out <dir>: write to <dir>/<stream>.ndjson
//   - --output / -o <path>: write to that path
//   - else: stdout
//
// Raw passthrough — sanctioned bypass of emit() per CLAUDE.md / plan P4.
func writeWellness(stream string, body io.Reader) error {
	if flagWellnessOutDir != "" {
		return output.WriteRaw(filepath.Join(flagWellnessOutDir, stream+".ndjson"), body)
	}
	if flagOutput != "" {
		return output.WriteRaw(flagOutput, body)
	}
	return output.WriteRawStdout(body)
}

func init() {
	wellnessCmd.PersistentFlags().StringVar(&flagWellnessSince, "since", "",
		"Cursor lower bound (unix ms, RFC3339, YYYY-MM-DD, duration like 7d/2h, or keyword today/yesterday/last-week/last-month/last-year; empty = all history)")
	wellnessCmd.PersistentFlags().StringVar(&flagWellnessOutDir, "out", "",
		"Write to <dir>/<stream>.ndjson instead of stdout")
	for _, s := range []endpoints.WellnessStream{
		endpoints.StreamSleep,
		endpoints.StreamActivity,
		endpoints.StreamRecovery,
		endpoints.StreamSleepStages,
	} {
		wellnessCmd.AddCommand(newWellnessStreamCmd(s))
	}
	rootCmd.AddCommand(wellnessCmd)
}
