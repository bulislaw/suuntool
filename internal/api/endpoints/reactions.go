package endpoints

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/bulislaw/suuntool/internal/api"
)

// Reaction is the reaction type — currently only "like" is observed in the wild.
type Reaction string

const ReactionLike Reaction = "like"

// AddReaction posts a reaction on a workout. The server is currently ignorant
// of the reaction type at the protocol level (the v1 endpoint just records "a
// reaction"); future reaction types may need a body payload. Caller MUST pass
// x-totp via headers (the cmd layer does this).
//
// Returns the response payload as RawMessage — the shape ranges from a string
// reaction-id to a richer object across captures.
func AddReaction(ctx context.Context, c *api.Client, workoutKey string, _ Reaction, headers map[string]string) (json.RawMessage, error) {
	merged := make(map[string]string, len(headers)+1)
	for k, v := range headers {
		merged[k] = v
	}
	merged["Content-Type"] = "application/json;charset=UTF-8"
	body, err := c.Do(ctx, "POST", "workouts/reaction/"+workoutKey, bytes.NewReader([]byte{}), merged)
	if err != nil {
		return nil, err
	}
	return api.DecodeAsko[json.RawMessage](body)
}

// RemoveReaction deletes the calling user's reaction from a workout.
func RemoveReaction(ctx context.Context, c *api.Client, workoutKey string) error {
	_, err := c.Do(ctx, "DELETE", "workouts/reaction/"+workoutKey, nil, nil)
	return err
}
