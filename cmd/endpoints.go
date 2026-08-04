package cmd

import (
	"github.com/spf13/cobra"
)

type endpointRow struct {
	Command    string `json:"command"`
	Method     string `json:"method"`
	Path       string `json:"path"`
	AuthNeeded bool   `json:"authNeeded"`
}

var endpointTable = []endpointRow{
	{"login", "POST", "/v1/login2", false},
	{"logout", "GET", "/v1/logout", true},
	{"whoami", "GET", "/v1/user", true},
	{"profile settings", "GET", "/v1/user/settings", true},
	{"profile follow", "GET", "/v1/user/follow", true},
	{"profile user <username>", "GET", "/v1/user/name/{username}", true},
	{"doctor", "GET", "/v1/servertime", false},
	{"workouts list", "GET", "/v1/workouts", true},
	{"workouts get <key>", "GET", "/v1/workouts/{key}", true},
	{"workouts count", "GET", "/v1/workouts/count", true},
	{"workouts stats [username]", "GET", "/v1/workouts/{username}/stats", true},
	{"workouts sml <key>", "GET", "/v1/workouts/{key}/sml", true},
	{"workouts fit <key>", "GET", "/v1/workout/exportFit/{key}", true},
	{"wellness sleep", "GET", "https://247.sports-tracker.com/v1/sleep/export", true},
	{"wellness activity", "GET", "https://247.sports-tracker.com/v1/activity/export", true},
	{"wellness recovery", "GET", "https://247.sports-tracker.com/v1/recovery/export", true},
	{"wellness sleepstages", "GET", "https://247.sports-tracker.com/v1/sleepstages/export", true},
	{"workouts comments <key>", "GET", "/v1/workouts/comments/{key}", true},
	{"workouts comment <key> [text]", "POST", "/v1/workouts/comment/{key}", true},
	{"workouts uncomment <comment-key>", "DELETE", "/v1/workouts/comment/{commentKey}", true},
	{"workouts react <key>", "POST", "/v1/workouts/reaction/{key}", true},
	{"workouts unreact <key>", "DELETE", "/v1/workouts/reaction/{key}", true},
	{"workouts edit <key>", "PUT", "/v1/workouts/{key}/attributes", true},
	{"workouts batch-update", "POST", "/v1/workouts/batchUpdate", true},
	{"workouts share <key>", "PUT", "/v1/workouts/{username}/{key}/share/{format}", true},
	{"workouts extensions <key>", "POST", "/v1/workout/extensions/{key}", true},
	{"workouts upload", "POST", "/v1/workout", true},
	{"workouts delete <key>", "DELETE", "/v1/workouts/{key}/delete", true},
	{"workouts export <key>", "GET", "(bundle: metadata + sml + fit + extensions + comments)", true},
	{"guides list", "GET", "/v1/suuntoplus/guides/items", true},
	{"guides download <id>", "GET", "/v1/suuntoplus/guides/files/{id}", true},
	{"guides upload <zip>", "POST", "/v1/suuntoplus/guides/files", true},
	{"guides update <id> <zip>", "PUT", "/v1/suuntoplus/guides/files/{id}", true},
	{"guides pin <id>", "PATCH", "/v1/suuntoplus/guides/items/{id}", true},
	{"guides unpin <id>", "PATCH", "/v1/suuntoplus/guides/items/{id}", true},
	{"guides priority", "GET", "/v1/suuntoplus/guides/priority", true},
}

var endpointsCmd = &cobra.Command{
	Use:    "endpoints",
	Short:  "Print the command→endpoint mapping (machine-readable with --format=json)",
	Hidden: true, // discoverable via `suuntool help endpoints`
	RunE: func(cmd *cobra.Command, args []string) error {
		return emit(endpointTable)
	},
}

func init() { rootCmd.AddCommand(endpointsCmd) }
