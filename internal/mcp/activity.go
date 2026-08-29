package mcp

import (
	"context"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bulislaw/suuntool/internal/api/endpoints"
)

// workoutWithName wraps a RemoteSyncedWorkout with an injected activityName
// so MCP clients don't have to look up the numeric activityId separately.
type workoutWithName struct {
	endpoints.RemoteSyncedWorkout
	ActivityName string `json:"activityName"`
}

type workoutDetailWithName struct {
	endpoints.RemoteSyncedWorkoutDetail
	ActivityName string `json:"activityName"`
}

// perActivityWithName mirrors PerActivityStats with the activity name injected.
type perActivityWithName struct {
	endpoints.PerActivityStats
	ActivityName string `json:"activityName"`
}

// workoutListView is the MCP-shaped response for workouts_list.
type workoutListView struct {
	Items []workoutWithName `json:"items"`
	Until int64             `json:"until"`
}

// workoutStatsView shadows AllStats with name-enriched rows. The remaining
// fields of WorkoutStats are surfaced via embedding.
type workoutStatsView struct {
	endpoints.WorkoutStats
	AllStats []perActivityWithName `json:"allStats"`
}

func enrichWorkout(w endpoints.RemoteSyncedWorkout) workoutWithName {
	return workoutWithName{RemoteSyncedWorkout: w, ActivityName: endpoints.ActivityName(w.ActivityID)}
}

func enrichWorkoutDetail(w endpoints.RemoteSyncedWorkoutDetail) workoutDetailWithName {
	return workoutDetailWithName{RemoteSyncedWorkoutDetail: w, ActivityName: endpoints.ActivityName(w.ActivityID)}
}

func enrichWorkoutList(l *endpoints.WorkoutList) *workoutListView {
	if l == nil {
		return nil
	}
	out := &workoutListView{Items: make([]workoutWithName, 0, len(l.Items)), Until: l.Until}
	for _, w := range l.Items {
		out.Items = append(out.Items, enrichWorkout(w))
	}
	return out
}

func enrichWorkoutStats(s *endpoints.WorkoutStats) *workoutStatsView {
	if s == nil {
		return nil
	}
	v := &workoutStatsView{WorkoutStats: *s, AllStats: make([]perActivityWithName, 0, len(s.AllStats))}
	for _, a := range s.AllStats {
		v.AllStats = append(v.AllStats, perActivityWithName{PerActivityStats: a, ActivityName: endpoints.ActivityName(a.ActivityID)})
	}
	return v
}

type activityNameArgs struct {
	ID int `json:"id" jsonschema:"numeric Suunto activityId (e.g. 1 = running, 2 = cycling)"`
}

type activityNameResult struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// registerActivityNameTool exposes endpoints.ActivityName as a stand-alone
// lookup tool. Unauthed — the table is embedded.
func registerActivityNameTool(s *sdkmcp.Server, _ *deps) {
	sdkmcp.AddTool(s, &sdkmcp.Tool{
		Name:        "activity_type_name",
		Description: "Look up the human-readable Suunto ActivityType name for a numeric activityId (e.g. 1 → RUNNING). Falls back to act=<id> for unknown ids.",
	}, func(ctx context.Context, req *sdkmcp.CallToolRequest, a activityNameArgs) (*sdkmcp.CallToolResult, any, error) {
		return nil, activityNameResult{ID: a.ID, Name: endpoints.ActivityName(a.ID)}, nil
	})
}
