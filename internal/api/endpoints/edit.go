package endpoints

import (
	"context"
	"encoding/json"

	"github.com/bulislaw/suuntool/internal/api"
)

// EditWorkout sends a partial-update JSON body to /v1/workouts/{key}/attributes.
// patch is the {field: value} map (typed as map[string]any so callers can pass
// numbers, strings, bools, nil to clear). Returns the server's response payload
// verbatim as RawMessage.
func EditWorkout(ctx context.Context, c *api.Client, workoutKey string, patch map[string]any) (json.RawMessage, error) {
	body, hdr, err := api.JSONBody(patch)
	if err != nil {
		return nil, &api.Error{Code: "USAGE", Message: "marshal patch: " + err.Error(), Exit: 2}
	}
	resp, err := c.Do(ctx, "PUT", "workouts/"+workoutKey+"/attributes", body, hdr)
	if err != nil {
		return nil, err
	}
	return api.DecodeAsko[json.RawMessage](resp)
}

// BatchUpdate posts a list of partial updates to /v1/workouts/batchUpdate. entries
// is the raw JSON array. Caller is responsible for shaping each entry (must have
// at minimum a "key" field per handoff §6.4); we accept []map[string]any rather
// than introducing a constrained type.
func BatchUpdate(ctx context.Context, c *api.Client, entries []map[string]any) (json.RawMessage, error) {
	body, hdr, err := api.JSONBody(entries)
	if err != nil {
		return nil, &api.Error{Code: "USAGE", Message: "marshal entries: " + err.Error(), Exit: 2}
	}
	resp, err := c.Do(ctx, "POST", "workouts/batchUpdate", body, hdr)
	if err != nil {
		return nil, err
	}
	return api.DecodeAsko[json.RawMessage](resp)
}
