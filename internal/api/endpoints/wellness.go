package endpoints

import (
	"context"
	"fmt"
	"io"

	"github.com/bulislaw/suuntool/internal/api"
)

// WellnessStream is one of the 4 valid streams.
type WellnessStream string

const (
	StreamSleep       WellnessStream = "sleep"
	StreamActivity    WellnessStream = "activity"
	StreamRecovery    WellnessStream = "recovery"
	StreamSleepStages WellnessStream = "sleepstages"
)

// FetchWellness streams gzipped NDJSON from
//
//	GET https://247.sports-tracker.com/v1/{stream}/export?since=<ms>
//
// The returned reader yields raw NDJSON (one JSON object per line) — gzip is
// unwrapped transparently by client.DoStream.
//
// Caller MUST Close the reader.
//
// Note: c must be constructed with api.NewTimelineClient — these paths are not
// on the ASKO base URL.
func FetchWellness(ctx context.Context, c *api.Client, stream WellnessStream, sinceMS int64) (io.ReadCloser, error) {
	path := fmt.Sprintf("v1/%s/export?since=%d", stream, sinceMS)
	return c.DoStream(ctx, "GET", path, nil, nil)
}
