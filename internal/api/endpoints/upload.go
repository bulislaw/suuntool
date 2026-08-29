package endpoints

import (
	"context"

	"github.com/bulislaw/suuntool/internal/api"
)

// UploadWorkout uploads an SML file (and optional extensions JSON) via POST /v1/workout.
// Returns the newly created workout (server-enriched).
//
// The body is built lazily by api.WorkoutMultipart and streamed through io.Pipe —
// the full SML file is NEVER buffered into memory.
func UploadWorkout(ctx context.Context, c *api.Client, smlPath, extensionsPath string) (*RemoteSyncedWorkout, error) {
	body, hdr, err := api.WorkoutMultipart(smlPath, extensionsPath)
	if err != nil {
		return nil, &api.Error{Code: "USAGE", Message: err.Error(), Exit: 2}
	}
	defer body.Close()
	resp, err := c.Do(ctx, "POST", "workout", body, hdr)
	if err != nil {
		return nil, err
	}
	w, err := api.DecodeAsko[RemoteSyncedWorkout](resp)
	if err != nil {
		return nil, err
	}
	return &w, nil
}
