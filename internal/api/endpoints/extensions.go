package endpoints

import (
	"context"
	"encoding/json"

	"github.com/bulislaw/suuntool/internal/api"
)

// DefaultExtensionTypes mirrors the set sent by the Android app.
var DefaultExtensionTypes = []string{
	"SummaryExtension", "FitnessExtension", "SkiExtension",
	"IntensityExtension", "DiveHeaderExtension",
	"SwimmingHeaderExtension", "WeatherExtension", "JumpRopeExtension",
}

// FetchExtensions hits POST /v1/workout/extensions/{key} with a JSON body listing
// the extension types the caller wants. Despite the POST verb this is a read —
// the server filters down to whatever the workout actually has.
//
// Returns the raw payload (server shape varies per workout/extension mix).
func FetchExtensions(ctx context.Context, c *api.Client, workoutKey string, types []string) (json.RawMessage, error) {
	if len(types) == 0 {
		types = DefaultExtensionTypes
	}
	body, hdr, err := api.JSONBody(types)
	if err != nil {
		return nil, &api.Error{Code: "USAGE", Message: "marshal types: " + err.Error(), Exit: 2}
	}
	resp, err := c.Do(ctx, "POST", "workout/extensions/"+workoutKey, body, hdr)
	if err != nil {
		return nil, err
	}
	return api.DecodeAsko[json.RawMessage](resp)
}
