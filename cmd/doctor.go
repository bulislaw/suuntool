package cmd

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bulislaw/suuntool/internal/api"
	"github.com/bulislaw/suuntool/internal/session"
)

type doctorReport struct {
	BaseURL       string `json:"baseURL"`
	SessionLoaded bool   `json:"sessionLoaded"`
	SessionPath   string `json:"sessionPath"`
	ServerTimeMS  int64  `json:"serverTimeMS,omitempty"`
	Note          string `json:"note,omitempty"`
}

func (d doctorReport) Pretty() string {
	out := fmt.Sprintf("baseURL       : %s\nsessionLoaded : %v\nsessionPath   : %s",
		d.BaseURL, d.SessionLoaded, d.SessionPath)
	if d.ServerTimeMS > 0 {
		out += fmt.Sprintf("\nserverTimeMS  : %d", d.ServerTimeMS)
	}
	if d.Note != "" {
		out += "\nnote          : " + d.Note
	}
	return out
}

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check connectivity and session validity",
	RunE: func(cmd *cobra.Command, args []string) error {
		rpt := doctorReport{BaseURL: baseURL(), SessionPath: session.Path()}
		if _, err := session.Load(); err == nil {
			rpt.SessionLoaded = true
		}
		c := api.NewClient(baseURL(), "", pickTimeout())
		ctx, cancel := context.WithTimeout(cmd.Context(), pickTimeout())
		defer cancel()
		body, err := c.Do(ctx, "GET", "servertime", nil, nil)
		if err != nil {
			rpt.Note = "servertime fetch failed: " + err.Error()
			_ = emit(rpt)
			return err
		}
		var st struct {
			ServerTime int64 `json:"servertime"`
		}
		if err := json.Unmarshal(body, &st); err == nil {
			rpt.ServerTimeMS = st.ServerTime
		}
		return emit(rpt)
	},
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}
