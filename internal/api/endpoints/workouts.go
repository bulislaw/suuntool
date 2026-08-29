package endpoints

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/bulislaw/suuntool/internal/api"
)

// LatLon is a latitude/longitude pair as returned by the workout API.
type LatLon struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// WorkoutTSS is the training stress score block embedded in workout responses.
// Field values may be absent depending on calculation method.
type WorkoutTSS struct {
	TrainingStressScore      *float64 `json:"trainingStressScore,omitempty"`
	CalculationMethod        *string  `json:"calculationMethod,omitempty"`
	IntensityFactor          *float64 `json:"intensityFactor,omitempty"`
	NormalizedPower          *float64 `json:"normalizedPower,omitempty"`
	AverageGradeAdjustedPace *float64 `json:"averageGradeAdjustedPace,omitempty"`
}

// WorkoutHRData is the heart-rate summary block embedded in workout responses.
type WorkoutHRData struct {
	Max          *float64 `json:"max,omitempty"`
	HRMax        *float64 `json:"hrmax,omitempty"`
	Avg          *float64 `json:"avg,omitempty"`
	UserMaxHR    *float64 `json:"userMaxHR,omitempty"`
	WorkoutAvgHR *float64 `json:"workoutAvgHR,omitempty"`
	WorkoutMaxHR *float64 `json:"workoutMaxHR,omitempty"`
}

// RemoteSyncedWorkout is the list-safe shape returned by /v1/workouts and
// /v1/workouts/{username}/public. It also contains fields shared with
// /v1/workouts/{key}; detail-only fields live in RemoteSyncedWorkoutDetail.
type RemoteSyncedWorkout struct {
	Key               string         `json:"key"`
	Username          string         `json:"username"`
	ActivityID        int            `json:"activityId"`
	StartTime         int64          `json:"startTime"`     // unix ms
	StopTime          int64          `json:"stopTime"`      // unix ms
	TotalTime         float64        `json:"totalTime"`     // seconds
	TotalDistance     float64        `json:"totalDistance"` // meters
	TotalAscent       float64        `json:"totalAscent"`
	TotalDescent      float64        `json:"totalDescent"`
	MaxSpeed          float64        `json:"maxSpeed,omitempty"`
	Polyline          string         `json:"polyline,omitempty"`
	StepCount         int            `json:"stepCount,omitempty"`
	RecoveryTime      int            `json:"recoveryTime,omitempty"`
	EnergyConsumption *float64       `json:"energyConsumption,omitempty"`
	HRData            *WorkoutHRData `json:"hrdata,omitempty"`
	TSS               *WorkoutTSS    `json:"tss,omitempty"`
	TSSList           []WorkoutTSS   `json:"tssList,omitempty"`
	StartPosition     *LatLon        `json:"startPosition,omitempty"`
	StopPosition      *LatLon        `json:"stopPosition,omitempty"`
	CenterPosition    *LatLon        `json:"centerPosition,omitempty"`
}

// RemoteSyncedWorkoutDetail adds heavier fields returned by /v1/workouts/{key}.
type RemoteSyncedWorkoutDetail struct {
	RemoteSyncedWorkout
	Extensions []json.RawMessage `json:"extensions,omitempty"`
}

// Pretty returns a single summary line for the workout.
func (w RemoteSyncedWorkout) Pretty() string {
	start := time.Unix(0, w.StartTime*int64(time.Millisecond)).UTC().Format(time.RFC3339)
	km := w.TotalDistance / 1000.0
	dur := formatDuration(w.TotalTime)
	return fmt.Sprintf("%s  %s  %s (act=%d)  %.2fkm  %s", w.Key, start, ActivityName(w.ActivityID), w.ActivityID, km, dur)
}

// WorkoutList wraps a page of workouts with the cursor metadata so JSON
// consumers see the pagination state.
type WorkoutList struct {
	Items []RemoteSyncedWorkout `json:"items"`
	Until int64                 `json:"until"` // metadata.until from the envelope
}

// WorkoutSummary is an aggregate over a set of workouts. Distance is meters
// and time is seconds (matching the wire types); Pretty() formats them.
type WorkoutSummary struct {
	Count         int                      `json:"count"`
	TotalDistance float64                  `json:"totalDistance"` // meters
	TotalTime     float64                  `json:"totalTime"`     // seconds
	TotalAscent   float64                  `json:"totalAscent"`
	TotalDescent  float64                  `json:"totalDescent"`
	ByActivity    map[int]PerActivityStats `json:"byActivity,omitempty"`
	// WeekOverWeek, when populated by SummaryWithWoW, is the per-activity
	// change in workout count between the trailing 7 days and the prior 7 days.
	WeekOverWeek map[int]ActivityDelta `json:"weekOverWeek,omitempty"`
}

// ActivityDelta is a week-over-week change row for one activity.
type ActivityDelta struct {
	Count int `json:"count"`
}

// DeltaColorizer maps a plain delta cell to a styled version. kind is one of
// "pos", "neg", "zero". May be nil (no styling).
type DeltaColorizer func(plain, kind string) string

// Summary computes aggregate totals over the items in the list.
func (l WorkoutList) Summary() WorkoutSummary {
	s := WorkoutSummary{Count: len(l.Items), ByActivity: map[int]PerActivityStats{}}
	for _, w := range l.Items {
		s.TotalDistance += w.TotalDistance
		s.TotalTime += w.TotalTime
		s.TotalAscent += w.TotalAscent
		s.TotalDescent += w.TotalDescent
		a := s.ByActivity[w.ActivityID]
		a.ActivityID = w.ActivityID
		a.Count++
		a.Distance += w.TotalDistance
		a.Duration += w.TotalTime
		s.ByActivity[w.ActivityID] = a
	}
	if len(s.ByActivity) == 0 {
		s.ByActivity = nil
	}
	return s
}

// Table returns the per-activity breakdown as headers + rows, sorted by
// activity ID. The aggregate scalars are surfaced by Pretty() as a header
// block — they aren't part of the table. If WeekOverWeek is populated, a
// trailing "ΔWoW" column is added (signed counts) so TSV consumers see it too.
func (s WorkoutSummary) Table() ([]string, [][]string) {
	headers := []string{"Act", "Type", "Count", "Distance", "Duration"}
	hasWoW := s.WeekOverWeek != nil
	if hasWoW {
		headers = append(headers, "ΔWoW")
	}
	ids := s.sortedActivityIDs()
	rows := make([][]string, 0, len(ids))
	for _, id := range ids {
		a := s.ByActivity[id]
		r := []string{
			fmt.Sprintf("%d", a.ActivityID),
			ActivityName(a.ActivityID),
			fmt.Sprintf("%d", a.Count),
			formatKm(a.Distance),
			formatDuration(a.Duration),
		}
		if hasWoW {
			r = append(r, formatDelta(s.WeekOverWeek[id].Count))
		}
		rows = append(rows, r)
	}
	return headers, rows
}

// sortedActivityIDs returns ByActivity's keys in ascending order. Shared by
// Table() and the colorizer so row index → activity-ID is consistent.
func (s WorkoutSummary) sortedActivityIDs() []int {
	ids := make([]int, 0, len(s.ByActivity))
	for id := range s.ByActivity {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return ids
}

// Pretty returns a multi-line summary of the aggregate, with the per-activity
// breakdown rendered as an aligned table.
func (s WorkoutSummary) Pretty() string { return s.RenderPretty(nil) }

// RenderPretty is Pretty with an optional colorizer applied to the ΔWoW cells
// when WeekOverWeek is populated. cmd-layer callers pass an ANSI colorizer
// when emitting to a TTY; passing nil produces uncolored output.
func (s WorkoutSummary) RenderPretty(color DeltaColorizer) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "workouts:  %d\n", s.Count)
	fmt.Fprintf(&sb, "distance:  %s\n", formatKm(s.TotalDistance))
	fmt.Fprintf(&sb, "time:      %s\n", formatDuration(s.TotalTime))
	fmt.Fprintf(&sb, "ascent:    %.0f m\n", s.TotalAscent)
	fmt.Fprintf(&sb, "descent:   %.0f m", s.TotalDescent)
	if len(s.ByActivity) > 0 {
		headers, rows := s.Table()
		var styler cellStyler
		if s.WeekOverWeek != nil && color != nil {
			ids := s.sortedActivityIDs()
			deltaCol := len(headers) - 1
			styler = func(col, row int, plain string) string {
				if col != deltaCol || row < 0 {
					return plain
				}
				d := s.WeekOverWeek[ids[row]].Count
				kind := "zero"
				switch {
				case d > 0:
					kind = "pos"
				case d < 0:
					kind = "neg"
				}
				return color(plain, kind)
			}
		}
		sb.WriteString("\n\nPer activity:\n")
		sb.WriteString(renderTableStyled(headers, rows, styler))
	}
	return sb.String()
}

// formatDelta renders an int with an explicit sign for non-negative values
// so the ΔWoW column reads at a glance: +3 / -2 / 0.
func formatDelta(d int) string {
	if d > 0 {
		return fmt.Sprintf("+%d", d)
	}
	return fmt.Sprintf("%d", d)
}

// SummaryWithWoW returns Summary() plus per-activity week-over-week deltas
// computed from l.Items. Items with startTime in [nowMS-7d, nowMS) are the
// "this week" bucket; items in [nowMS-14d, nowMS-7d) are the "previous week"
// bucket; older items are ignored for the delta (but still counted in the
// aggregate totals). nowMS == 0 falls back to time.Now().UnixMilli().
func (l WorkoutList) SummaryWithWoW(nowMS int64) WorkoutSummary {
	s := l.Summary()
	if nowMS == 0 {
		nowMS = time.Now().UnixMilli()
	}
	const weekMS = int64(7 * 24 * 3600 * 1000)
	thisStart := nowMS - weekMS
	prevStart := nowMS - 2*weekMS
	type bucket struct{ this, prev int }
	by := map[int]*bucket{}
	for _, w := range l.Items {
		switch {
		case w.StartTime >= thisStart && w.StartTime < nowMS:
			b := by[w.ActivityID]
			if b == nil {
				b = &bucket{}
				by[w.ActivityID] = b
			}
			b.this++
		case w.StartTime >= prevStart && w.StartTime < thisStart:
			b := by[w.ActivityID]
			if b == nil {
				b = &bucket{}
				by[w.ActivityID] = b
			}
			b.prev++
		}
	}
	if len(by) == 0 {
		return s
	}
	s.WeekOverWeek = make(map[int]ActivityDelta, len(by))
	for id, b := range by {
		s.WeekOverWeek[id] = ActivityDelta{Count: b.this - b.prev}
	}
	return s
}

// Table returns the workout page as headers + rows. Used by Pretty() for the
// aligned TTY table and by --format tsv for machine consumption.
func (l WorkoutList) Table() ([]string, [][]string) {
	headers := []string{"Date", "Act", "Type", "Distance", "Duration", "Ascent", "Key"}
	rows := make([][]string, 0, len(l.Items))
	for _, w := range l.Items {
		rows = append(rows, []string{
			time.Unix(0, w.StartTime*int64(time.Millisecond)).Local().Format("2006-01-02 15:04"),
			fmt.Sprintf("%d", w.ActivityID),
			ActivityName(w.ActivityID),
			fmt.Sprintf("%.2f km", w.TotalDistance/1000.0),
			formatDuration(w.TotalTime),
			fmt.Sprintf("%.0f m", w.TotalAscent),
			w.Key,
		})
	}
	return headers, rows
}

// Pretty renders the workout page as a fixed-width table with an aggregate
// footer (count, distance, time) so the human render is self-summarizing.
// Empty list still emits a header row + footer so the output is recognisable.
func (l WorkoutList) Pretty() string {
	headers, rows := l.Table()
	s := l.Summary()
	footer := fmt.Sprintf("\n%d %s  %s  %s",
		s.Count, pluralWorkout(s.Count), formatKm(s.TotalDistance), formatDuration(s.TotalTime))
	if l.Until > 0 {
		footer += fmt.Sprintf("  (until=%d)", l.Until)
	}
	return renderTable(headers, rows) + footer
}

func pluralWorkout(n int) string {
	if n == 1 {
		return "workout"
	}
	return "workouts"
}

// ListWorkoutsOpts controls pagination for ListWorkouts.
type ListWorkoutsOpts struct {
	Since  int64 // unix ms; 0 = all
	Limit  int   // server max 100; if 0, default 20
	Offset int
}

// ListWorkouts fetches a single page of workouts. The caller paginates by
// reading Until and re-calling with Since=Until (or Offset+=Limit).
func ListWorkouts(ctx context.Context, c *api.Client, opts ListWorkoutsOpts) (*WorkoutList, error) {
	limit := opts.Limit
	if limit == 0 {
		limit = 20
	}

	path := fmt.Sprintf("workouts?since=%d&limit=%d&offset=%d", opts.Since, limit, opts.Offset)

	body, err := c.Do(ctx, "GET", path, nil, nil)
	if err != nil {
		return nil, err
	}

	items, rawMeta, err := api.DecodeAskoWithMeta[[]RemoteSyncedWorkout](body)
	if err != nil {
		return nil, err
	}

	var until int64
	if len(rawMeta) > 0 {
		var meta struct {
			Until int64 `json:"until"`
		}
		_ = json.Unmarshal(rawMeta, &meta)
		until = meta.Until
	}

	if items == nil {
		items = []RemoteSyncedWorkout{}
	}

	return &WorkoutList{Items: items, Until: until}, nil
}

// GetWorkout fetches a single workout by key.
func GetWorkout(ctx context.Context, c *api.Client, key string) (*RemoteSyncedWorkoutDetail, error) {
	body, err := c.Do(ctx, "GET", "workouts/"+key, nil, nil)
	if err != nil {
		return nil, err
	}
	w, err := api.DecodeAsko[RemoteSyncedWorkoutDetail](body)
	if err != nil {
		return nil, err
	}
	return &w, nil
}

// WorkoutCount is returned by /v1/workouts/count.
type WorkoutCount struct {
	Count      int `json:"count"`
	TotalCount int `json:"totalCount"`
}

// Pretty returns a two-line key/value representation.
func (wc WorkoutCount) Pretty() string {
	return fmt.Sprintf("count:       %d\ntotalCount:  %d", wc.Count, wc.TotalCount)
}

// PerActivityStats is one row of WorkoutStats.AllStats.
type PerActivityStats struct {
	ActivityID int     `json:"activityId"`
	Count      int     `json:"count"`
	Distance   float64 `json:"distance"`
	Duration   float64 `json:"duration"`
	Energy     float64 `json:"energy"`
}

// WorkoutStats is the response from /v1/workouts/{username}/stats.
// Field names follow what Suunto returns; numerics are kept verbatim.
type WorkoutStats struct {
	TotalDistanceSum          float64            `json:"totalDistanceSum"`
	TotalTimeSum              float64            `json:"totalTimeSum"`
	TotalEnergyConsumptionSum float64            `json:"totalEnergyConsumptionSum"`
	TotalNumberOfWorkoutsSum  int                `json:"totalNumberOfWorkoutsSum"`
	TotalDays                 int                `json:"totalDays"`
	AllStats                  []PerActivityStats `json:"allStats"`
}

// Table returns the per-activity breakdown as headers + rows. The aggregate
// scalars (totalDistance/Time/Energy/Days) live on s and are surfaced by
// Pretty() as a header block — they aren't part of the table.
func (s WorkoutStats) Table() ([]string, [][]string) {
	headers := []string{"Act", "Type", "Count", "Distance", "Duration", "Energy"}
	rows := make([][]string, 0, len(s.AllStats))
	for _, a := range s.AllStats {
		rows = append(rows, []string{
			fmt.Sprintf("%d", a.ActivityID),
			ActivityName(a.ActivityID),
			fmt.Sprintf("%d", a.Count),
			formatKm(a.Distance),
			formatDuration(a.Duration),
			fmt.Sprintf("%.0f kcal", a.Energy),
		})
	}
	return headers, rows
}

// Pretty returns a multi-line summary of the aggregate stats, with per-activity
// rows rendered as an aligned table.
func (s WorkoutStats) Pretty() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "workouts:  %d\n", s.TotalNumberOfWorkoutsSum)
	fmt.Fprintf(&sb, "distance:  %s\n", formatKm(s.TotalDistanceSum))
	fmt.Fprintf(&sb, "time:      %s\n", formatDuration(s.TotalTimeSum))
	fmt.Fprintf(&sb, "energy:    %.0f kcal\n", s.TotalEnergyConsumptionSum)
	fmt.Fprintf(&sb, "days:      %d", s.TotalDays)
	if len(s.AllStats) > 0 {
		headers, rows := s.Table()
		sb.WriteString("\n\nPer activity:\n")
		sb.WriteString(renderTable(headers, rows))
	}
	return sb.String()
}

// Stats fetches /v1/workouts/{username}/stats. Empty username is rejected at the
// caller (cmd layer falls back to the session username).
func Stats(ctx context.Context, c *api.Client, username string) (*WorkoutStats, error) {
	path := fmt.Sprintf("workouts/%s/stats", username)
	body, err := c.Do(ctx, "GET", path, nil, nil)
	if err != nil {
		return nil, err
	}
	ws, err := api.DecodeAsko[WorkoutStats](body)
	if err != nil {
		return nil, err
	}
	return &ws, nil
}

// FetchSML returns the full /v1/workouts/{key}/sml body as a stream. Despite the
// path name, the body is application/json (~5MB per workout). Caller MUST Close
// the reader.
func FetchSML(ctx context.Context, c *api.Client, key string) (io.ReadCloser, error) {
	return c.DoStream(ctx, "GET", "workouts/"+key+"/sml", nil, nil)
}

// FetchFIT returns the binary .fit export from /v1/workout/exportFit/{key}.
// Note the singular "workout/" prefix per handoff §6.19. application/octet-stream.
// Caller MUST Close.
func FetchFIT(ctx context.Context, c *api.Client, key string) (io.ReadCloser, error) {
	return c.DoStream(ctx, "GET", "workout/exportFit/"+key, nil, nil)
}

// CountWorkouts hits /v1/workouts/count. Both until and sharingFlags are
// required by the server (handoff §5 quirks). If untilMS <= 0, the caller
// should pass auth.NowMS().
func CountWorkouts(ctx context.Context, c *api.Client, untilMS int64, sharingFlags int) (*WorkoutCount, error) {
	path := fmt.Sprintf("workouts/count?until=%d&sharingFlags=%d", untilMS, sharingFlags)
	body, err := c.Do(ctx, "GET", path, nil, nil)
	if err != nil {
		return nil, err
	}
	wc, err := api.DecodeAsko[WorkoutCount](body)
	if err != nil {
		return nil, err
	}
	return &wc, nil
}

// DeleteWorkout permanently removes a workout. Returns nil on success.
// Note the path has a trailing /delete (not just /workouts/{key}) — see handoff §6.4.
func DeleteWorkout(ctx context.Context, c *api.Client, workoutKey string) error {
	_, err := c.Do(ctx, "DELETE", "workouts/"+workoutKey+"/delete", nil, nil)
	return err
}
