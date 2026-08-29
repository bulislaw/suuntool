package endpoints_test

import (
	"strings"
	"testing"

	"github.com/bulislaw/suuntool/internal/api/endpoints"
)

func TestWorkoutList_Pretty_TableShape(t *testing.T) {
	l := endpoints.WorkoutList{
		Items: []endpoints.RemoteSyncedWorkout{
			{Key: "wk1", ActivityID: 3, StartTime: 1700000000000, TotalTime: 3661, TotalDistance: 12345.6, TotalAscent: 120},
			{Key: "wk2", ActivityID: 5, StartTime: 1700100000000, TotalTime: 600, TotalDistance: 1500, TotalAscent: 0},
		},
		Until: 1700100000000,
	}
	out := l.Pretty()
	for _, want := range []string{"Date", "Act", "Distance", "Duration", "Ascent", "Key", "wk1", "wk2", "2 workouts", "until=1700100000000", "─"} {
		if !strings.Contains(out, want) {
			t.Errorf("Pretty() missing %q\n---\n%s\n---", want, out)
		}
	}
}

func TestWorkoutList_Pretty_SingularFooter(t *testing.T) {
	l := endpoints.WorkoutList{Items: []endpoints.RemoteSyncedWorkout{{Key: "wk1"}}}
	if !strings.Contains(l.Pretty(), "1 workout ") {
		t.Errorf("expected '1 workout' singular footer, got:\n%s", l.Pretty())
	}
}

func TestWorkoutStats_Pretty_PerActivityTable(t *testing.T) {
	s := endpoints.WorkoutStats{
		TotalNumberOfWorkoutsSum:  10,
		TotalDistanceSum:          12000,
		TotalTimeSum:              3600,
		TotalEnergyConsumptionSum: 2500,
		TotalDays:                 5,
		AllStats: []endpoints.PerActivityStats{
			{ActivityID: 3, Count: 4, Distance: 8000, Duration: 1800, Energy: 1200},
			{ActivityID: 5, Count: 6, Distance: 4000, Duration: 1800, Energy: 1300},
		},
	}
	out := s.Pretty()
	for _, want := range []string{"workouts:", "Per activity:", "Act", "Count", "Distance", "kcal", "─"} {
		if !strings.Contains(out, want) {
			t.Errorf("Pretty() missing %q\n---\n%s\n---", want, out)
		}
	}
}

func TestCommentList_Pretty_TableShape(t *testing.T) {
	l := endpoints.CommentList{
		Items: []endpoints.Comment{
			{Key: "c1", Comment: "great run", Username: "alice", Timestamp: 1700000000000},
			{Key: "c2", Comment: strings.Repeat("x", 200), Username: "", Timestamp: 1700100000000},
		},
	}
	out := l.Pretty()
	for _, want := range []string{"Time", "User", "Comment", "Key", "alice", "(unknown)", "great run", "…", "2 comments"} {
		if !strings.Contains(out, want) {
			t.Errorf("Pretty() missing %q\n---\n%s\n---", want, out)
		}
	}
}
