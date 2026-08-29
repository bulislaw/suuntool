package endpoints

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/bulislaw/suuntool/internal/api"
)

// ServerTime is the unauthenticated GET /v1/servertime probe used by `doctor`.
type ServerTime struct {
	ServerTimeMS int64 `json:"servertime"`
}

// Pretty implements output.Prettier.
func (s ServerTime) Pretty() string {
	return fmt.Sprintf("serverTimeMS : %d", s.ServerTimeMS)
}

// FetchServerTime probes GET /v1/servertime. No envelope, no auth.
func FetchServerTime(ctx context.Context, c *api.Client) (*ServerTime, error) {
	body, err := c.Do(ctx, "GET", "servertime", nil, nil)
	if err != nil {
		return nil, err
	}
	var s ServerTime
	if err := json.Unmarshal(body, &s); err != nil {
		return nil, &api.Error{Code: "DECODE", Message: err.Error(), Exit: 5}
	}
	return &s, nil
}
