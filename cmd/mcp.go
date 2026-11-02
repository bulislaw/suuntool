package cmd

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/bulislaw/suuntool/internal/api"
	"github.com/bulislaw/suuntool/internal/mcp"
	"github.com/spf13/cobra"
)

var (
	flagMCPAllowWrite       bool
	flagMCPAllowDestructive bool
	flagMCPListen           string
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Run an MCP server over stdio or Streamable HTTP",
	Long: `Run an MCP (Model Context Protocol) server that exposes the same
endpoints suuntool's other subcommands use as MCP tools. Intended to be wired
into MCP-capable clients like Claude Desktop or Claude Code so an LLM can call
Suunto endpoints directly with structured arguments.

By default only read tools are exposed. --allow-write adds comment/react/edit/
share/extensions/upload. --allow-destructive (requires --allow-write) adds
delete/uncomment/unreact.

Stdio is the default. --listen :8080 serves Streamable HTTP at /mcp and a
liveness endpoint at /healthz. HTTP requires SUUNTOOL_MCP_TOKEN; clients must
send Authorization: Bearer <token>. Use a trusted LAN/VPN or an HTTPS proxy.`,
	Example: `  # default: read-only tools
  suuntool mcp

  # allow comments, reactions, edits
  suuntool mcp --allow-write

  # additionally allow deletes
  suuntool mcp --allow-write --allow-destructive

  # Claude Desktop config snippet:
  #   "mcpServers": { "suuntool": { "command": "suuntool", "args": ["mcp"] } }`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if flagMCPAllowDestructive && !flagMCPAllowWrite {
			return &api.Error{
				Code:    "USAGE",
				Message: "--allow-destructive requires --allow-write",
				Hint:    "Add --allow-write or drop --allow-destructive",
				Exit:    2,
			}
		}
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		return mcp.Run(ctx, mcp.Opts{
			AllowWrite:       flagMCPAllowWrite,
			AllowDestructive: flagMCPAllowDestructive,
			BaseURL:          baseURL(),
			Timeout:          pickTimeout(),
			Verbose:          flagVerbose,
			Listen:           flagMCPListen,
			BearerToken:      os.Getenv("SUUNTOOL_MCP_TOKEN"),
		})
	},
}

func init() {
	mcpCmd.Flags().StringVar(&flagMCPListen, "listen", "", "serve Streamable HTTP at this address instead of stdio (e.g. :8080); requires SUUNTOOL_MCP_TOKEN")
	mcpCmd.Flags().BoolVar(&flagMCPAllowWrite, "allow-write", false, "expose POST/PUT tools (comments, reactions, edits, share, upload)")
	mcpCmd.Flags().BoolVar(&flagMCPAllowDestructive, "allow-destructive", false, "additionally expose DELETE tools (delete workout, uncomment, unreact); requires --allow-write")
	rootCmd.AddCommand(mcpCmd)
}
