package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/bulislaw/suuntool/internal/cache"
)

var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Inspect or clear locally cached download artifacts",
	Long: `Inspect or clear the automatic, per-account cache for downloaded SML,
FIT, and guide archive files. These commands do not contact the Suunto server.`,
}

type cacheStatusView struct{ cache.Status }

func (v cacheStatusView) Pretty() string {
	out := fmt.Sprintf("cache entries: %d\ncache size:    %d bytes", v.Entries, v.Bytes)
	if !v.NewestAt.IsZero() {
		out += "\nnewest entry:  " + v.NewestAt.Local().Format(time.RFC3339)
	}
	for _, kind := range v.Kinds {
		out += fmt.Sprintf("\n%s: %d entries, %d bytes", kind.Kind, kind.Entries, kind.Bytes)
	}
	return out
}

var cacheStatusCmd = &cobra.Command{
	Use:     "status",
	Short:   "Show the current account's local cache usage",
	Example: "  suuntool cache status",
	RunE: func(cmd *cobra.Command, args []string) error {
		_, sess, err := authedClient()
		if err != nil {
			return err
		}
		store, err := openCacheStore(sess)
		if err != nil {
			return err
		}
		status, err := store.Status()
		if err != nil {
			return err
		}
		return emit(cacheStatusView{Status: status})
	},
}

type cacheClearResult struct {
	Cleared bool `json:"cleared"`
}

func (r cacheClearResult) Pretty() string { return "Cache cleared." }

var cacheClearCmd = &cobra.Command{
	Use:     "clear",
	Short:   "Clear the current account's local cache",
	Example: "  suuntool cache clear",
	RunE: func(cmd *cobra.Command, args []string) error {
		_, sess, err := authedClient()
		if err != nil {
			return err
		}
		store, err := openCacheStore(sess)
		if err != nil {
			return err
		}
		if err := store.Clear(); err != nil {
			return err
		}
		return emit(cacheClearResult{Cleared: true})
	},
}

func init() {
	cacheCmd.AddCommand(cacheStatusCmd, cacheClearCmd)
	rootCmd.AddCommand(cacheCmd)
}
