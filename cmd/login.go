package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/tajchert/suuntool/internal/api/endpoints"
	"github.com/tajchert/suuntool/internal/session"
)

var (
	flagLoginEmail         string
	flagLoginPasswordStdin bool
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate and persist a session",
	Long: `Authenticate with the Suunto / Sports-Tracker API and save the session.

The session is stored at the XDG-compliant path:
  $XDG_CONFIG_HOME/suuntool/session.json
or ~/.config/suuntool/session.json by default.
Override with SUUNTOOL_SESSION_FILE.

Password input:
  By default the password is read interactively (hidden prompt).
  Pass --password-stdin to read from stdin, useful in scripts:

    echo "hunter2" | suuntool login --email user@example.com --password-stdin
    cat pass.txt   | suuntool login --email user@example.com --password-stdin`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(cmd.Context(), pickTimeout())
		defer cancel()

		var pwd string
		if flagLoginPasswordStdin {
			scanner := bufio.NewScanner(os.Stdin)
			if scanner.Scan() {
				pwd = strings.TrimRight(scanner.Text(), "\r\n")
			}
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("reading password from stdin: %w", err)
			}
		} else {
			fd := int(os.Stdin.Fd())
			if !term.IsTerminal(fd) {
				return fmt.Errorf("stdin is not a TTY; use --password-stdin to pipe password")
			}
			fmt.Fprint(os.Stderr, "Password: ")
			raw, err := term.ReadPassword(fd)
			fmt.Fprintln(os.Stderr)
			if err != nil {
				return fmt.Errorf("reading password: %w", err)
			}
			pwd = string(raw)
		}

		remote, err := endpoints.Login(ctx, baseURL(), flagLoginEmail, pwd)
		if err != nil {
			return err
		}

		sess := &session.Session{
			SessionKey: remote.SessionKey,
			Username:   remote.Username,
			Email:      remote.Email,
			UserKey:    remote.UserKey,
			Country:    remote.Country,
		}
		if err := session.Save(sess); err != nil {
			return err
		}

		if !flagQuiet {
			fmt.Fprintf(os.Stderr, "Logged in as %s (%s). Session saved to %s.\n",
				sess.Username, sess.Email, session.Path())
		}

		return emit(remote)
	},
}

func init() {
	lf := loginCmd.Flags()
	lf.StringVar(&flagLoginEmail, "email", "", "Suunto / Sports-Tracker account email (required)")
	_ = loginCmd.MarkFlagRequired("email")
	lf.BoolVar(&flagLoginPasswordStdin, "password-stdin", false, "Read password from stdin instead of prompting")

	rootCmd.AddCommand(loginCmd)
}
