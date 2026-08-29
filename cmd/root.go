package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/term"

	"github.com/tajchert/suuntool/internal/api"
	"github.com/tajchert/suuntool/internal/cache"
	"github.com/tajchert/suuntool/internal/metrics"
	"github.com/tajchert/suuntool/internal/output"
	"github.com/tajchert/suuntool/internal/session"
)

// Exit codes — stable, documented in --help.
const (
	ExitOK        = 0
	ExitGeneric   = 1
	ExitUsage     = 2
	ExitNetwork   = 3
	ExitAuth      = 4
	ExitServer    = 5
	ExitNotFound  = 6
	ExitForbidden = 7
)

var (
	flagOutput  string
	flagFormat  string
	flagQuiet   bool
	flagVerbose bool
	flagNoColor bool
	flagTimeout time.Duration
	flagConfig  string
	flagFields  []string
)

var openCacheStore = cache.New

var rootCmd = &cobra.Command{
	Use:   "suuntool",
	Short: "Unofficial CLI for the Suunto / Sports-Tracker API",
	Long: `suuntool is a command-line client for the (unofficial, reverse-engineered)
Suunto / Sports-Tracker API. v1 covers login and profile read-side endpoints.

Environment variables:
  SUUNTOOL_SESSION_FILE  Override session storage path
  SUUNTOOL_FORMAT        Default output format (json|pretty|tsv|auto)
  SUUNTOOL_TIMEOUT       Default HTTP timeout (e.g. 30s)
  NO_COLOR               Disable ANSI styling

Exit codes:
  0 ok   1 generic   2 usage   3 network   4 auth
  5 server   6 not-found   7 forbidden`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Cobra child commands do not inherit the root command's context, so
		// attach counters at the executed command boundary.
		if metrics.FromContext(cmd.Context()) == nil {
			cmd.SetContext(metrics.WithCounters(cmd.Context(), metrics.New()))
		}
	},
}

func Execute() {
	executed, err := rootCmd.ExecuteC()
	if flagVerbose && (executed == nil || executed.Name() != "mcp") {
		var s metrics.Snapshot
		if executed != nil {
			s = metrics.FromContext(executed.Context()).Snapshot()
		}
		fmt.Fprintf(os.Stderr, "suuntool: server requests=%d, cache hits=%d\n", s.ServerRequests, s.CacheHits)
	}
	if err != nil {
		code := ExitGeneric
		if c, ok := err.(interface{ ExitCode() int }); ok {
			code = c.ExitCode()
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(code)
	}
}

func init() {
	pf := rootCmd.PersistentFlags()
	pf.StringVarP(&flagOutput, "output", "o", "", "Write to file (format from extension or --format)")
	pf.StringVarP(&flagFormat, "format", "f", "auto", "Output format: json|pretty|tsv|auto")
	pf.BoolVarP(&flagQuiet, "quiet", "q", false, "Suppress non-error logs")
	pf.BoolVarP(&flagVerbose, "verbose", "v", false, "Verbose (debug) logs to stderr")
	pf.BoolVar(&flagNoColor, "no-color", false, "Disable ANSI styling")
	pf.DurationVar(&flagTimeout, "timeout", 30*time.Second, "HTTP timeout")
	pf.StringVar(&flagConfig, "config", "", "Path to config file")
	pf.StringSliceVar(&flagFields, "fields", nil, "Project output to these JSON fields (e.g. key,startTime). Forces JSON.")

	_ = viper.BindPFlag("format", pf.Lookup("format"))
	_ = viper.BindPFlag("timeout", pf.Lookup("timeout"))
	viper.SetEnvPrefix("SUUNTOOL")
	viper.AutomaticEnv()
}

// baseURL returns the API base URL, honoring SUUNTOOL_BASE_URL env var.
func baseURL() string {
	if u := os.Getenv("SUUNTOOL_BASE_URL"); u != "" {
		return u
	}
	return api.DefaultBaseURL
}

// authedClient loads the saved session and returns an authenticated client.
// Returns *api.Error{Code:"AUTH_EXPIRED"} when no session is on disk.
func authedClient() (*api.Client, *session.Session, error) {
	s, err := session.Load()
	if err != nil {
		if errors.Is(err, session.ErrNoSession) {
			return nil, nil, &api.Error{
				Code:    "AUTH_EXPIRED",
				Message: "no saved session",
				Hint:    "Run: suuntool login",
				Exit:    ExitAuth,
			}
		}
		return nil, nil, err
	}
	c := api.NewClient(baseURL(), s.SessionKey, flagTimeout)
	return c, s, nil
}

// cachedArtifact serves one lossless download from the per-account cache when
// available. Cache setup failures deliberately degrade to a normal network
// request: local caching must never hide usable server data.
func cachedArtifact(ctx context.Context, s *session.Session, kind cache.Kind, id string, fetch func() (io.ReadCloser, error)) (io.ReadCloser, error) {
	store, err := openCacheStore(s)
	if err != nil {
		store = nil
	}
	return cache.Cached(ctx, store, kind, id, fetch)
}

// invalidateWorkoutArtifacts is best effort. A failure here must not turn a
// successfully applied server-side edit into a failed command.
func invalidateWorkoutArtifacts(s *session.Session, key string) {
	store, err := openCacheStore(s)
	if err != nil {
		return
	}
	_ = store.Remove(cache.WorkoutSML, key)
	_ = store.Remove(cache.WorkoutFIT, key)
}

func invalidateGuideArchive(s *session.Session, id string) {
	store, err := openCacheStore(s)
	if err == nil {
		_ = store.Remove(cache.GuideArchive, id)
	}
}

// renderOpts builds output.Opts from current flags.
func renderOpts() output.Opts {
	return output.Opts{
		Format: flagFormat,
		IsTTY:  output.IsStdoutTTY(),
		Fields: flagFields,
	}
}

// emit writes v to the output file (if --output is set) or stdout.
func emit(v any) error {
	if flagOutput != "" {
		return output.RenderToFile(flagOutput, v, renderOpts())
	}
	return output.Render(os.Stdout, v, renderOpts())
}

// pickTimeout returns flagTimeout if >0, else 30 seconds.
func pickTimeout() time.Duration {
	if flagTimeout > 0 {
		return flagTimeout
	}
	return 30 * time.Second
}

// mergeHeaders returns a new map combining a and b (b wins on collision).
// Used to merge totp + Content-Type without mutating shared maps.
func mergeHeaders(a, b map[string]string) map[string]string {
	out := make(map[string]string, len(a)+len(b))
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}

// confirm prompts the user for a y/N confirmation. Behavior:
//   - yes == true: immediately returns true (used by --yes flags).
//   - stdin is a TTY: writes prompt + " [y/N] " to stderr, reads a line,
//     returns true only on case-insensitive "y" or "yes".
//   - stdin is NOT a TTY and yes == false: returns a *api.Error{Code:"USAGE", Exit:2}.
//     This prevents agents/scripts from accidentally bypassing destructive
//     operations by piping nothing into stdin.
func confirm(prompt string, yes bool) (bool, error) {
	if yes {
		return true, nil
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return false, &api.Error{
			Code:    "USAGE",
			Message: "refusing to run destructive operation without --yes on a non-TTY",
			Hint:    "Pass --yes to confirm non-interactively",
			Exit:    ExitUsage,
		}
	}
	fmt.Fprintf(os.Stderr, "%s [y/N] ", prompt)
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return false, nil
	}
	ans := strings.ToLower(strings.TrimSpace(scanner.Text()))
	return ans == "y" || ans == "yes", nil
}
