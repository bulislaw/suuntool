package cmd

import (
	"github.com/spf13/cobra"
	"github.com/bulislaw/suuntool/internal/api"
	"github.com/bulislaw/suuntool/internal/mcp"
)

var (
	flagMCPAllowWrite       bool
	flagMCPAllowDestructive bool
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Run an MCP server over stdio that exposes suuntool endpoints as tools",
	Long: `Run an MCP (Model Context Protocol) server over stdio that exposes the same
endpoints suuntool's other subcommands use as MCP tools. Intended to be wired
into MCP-capable clients like Claude Desktop or Claude Code so an LLM can call
Suunto endpoints directly with structured arguments.

By default only read tools are exposed. --allow-write adds comment/react/edit/
share/extensions/upload. --allow-destructive (requires --allow-write) adds
delete/uncomment/unreact.`,
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
		return mcp.Run(cmd.Context(), mcp.Opts{
			AllowWrite:       flagMCPAllowWrite,
			AllowDestructive: flagMCPAllowDestructive,
			BaseURL:          baseURL(),
			Timeout:          pickTimeout(),
			Verbose:          flagVerbose,
		})
	},
}

func init() {
	mcpCmd.Flags().BoolVar(&flagMCPAllowWrite, "allow-write", false, "expose POST/PUT tools (comments, reactions, edits, share, upload)")
	mcpCmd.Flags().BoolVar(&flagMCPAllowDestructive, "allow-destructive", false, "additionally expose DELETE tools (delete workout, uncomment, unreact); requires --allow-write")
	rootCmd.AddCommand(mcpCmd)
}
