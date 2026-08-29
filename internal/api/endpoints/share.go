package endpoints

import (
	"context"

	"github.com/bulislaw/suuntool/internal/api"
)

// ShareFormat is the GPX share format.
type ShareFormat string

const (
	ShareGPXRoute ShareFormat = "gpx-route"
	ShareGPXTrack ShareFormat = "gpx-track"
)

// ShareWorkout requests a signed GPX share URL for a workout. Returns the URL
// string from payload.
func ShareWorkout(ctx context.Context, c *api.Client, username, workoutKey string, format ShareFormat) (string, error) {
	headers := map[string]string{"Brand": "suuntoapp"}
	resp, err := c.Do(ctx, "PUT",
		"workouts/"+username+"/"+workoutKey+"/share/"+string(format),
		nil, headers)
	if err != nil {
		return "", err
	}
	url, err := api.DecodeAsko[string](resp)
	if err != nil {
		return "", err
	}
	return url, nil
}
