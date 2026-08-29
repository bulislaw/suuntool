package endpoints_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bulislaw/suuntool/internal/api"
	"github.com/bulislaw/suuntool/internal/api/endpoints"
	"github.com/bulislaw/suuntool/internal/output"
)

func TestListWorkouts_DecodesEnvelopeAndCursor(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/workouts", r.URL.Path)
		assert.Equal(t, "SK", r.Header.Get("STTAuthorization"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"error": null,
			"payload": [{"key":"wk1","username":"alice","activityId":1,"startTime":1700000000000,"totalDistance":5000.0,"totalTime":1800.0}],
			"metadata": {"until":1700001000000}
		}`))
	}))
	defer srv.Close()

	client := api.NewClient(srv.URL+"/v1/", "SK", 0)
	list, err := endpoints.ListWorkouts(context.Background(), client, endpoints.ListWorkoutsOpts{})
	require.NoError(t, err)
	require.NotNil(t, list)
	require.Len(t, list.Items, 1)
	assert.Equal(t, "wk1", list.Items[0].Key)
	assert.Equal(t, "alice", list.Items[0].Username)
	assert.Equal(t, int64(1700001000000), list.Until)
}

func TestGetWorkout_ParsesSingle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/workouts/wk1", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"error": null,
			"payload": {"key":"wk1","username":"alice","activityId":1,"startTime":1700000000000,"totalDistance":5000.0,"totalTime":1800.0}
		}`))
	}))
	defer srv.Close()

	client := api.NewClient(srv.URL+"/v1/", "SK", 0)
	w, err := endpoints.GetWorkout(context.Background(), client, "wk1")
	require.NoError(t, err)
	require.NotNil(t, w)
	assert.Equal(t, "wk1", w.Key)
	assert.Equal(t, "alice", w.Username)
	assert.Equal(t, 1, w.ActivityID)
	assert.InDelta(t, 5000.0, w.TotalDistance, 0.001)
}

func TestGetWorkout_PreservesOptionalMetricsAndExtensions(t *testing.T) {
	fixture := readFixture(t, "workout_detail_extra_fields.json")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/workouts/wk1", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(fixture)
	}))
	defer srv.Close()

	client := api.NewClient(srv.URL+"/v1/", "SK", 0)
	w, err := endpoints.GetWorkout(context.Background(), client, "wk1")
	require.NoError(t, err)
	require.NotNil(t, w)

	require.NotNil(t, w.EnergyConsumption)
	assert.InDelta(t, 321.5, *w.EnergyConsumption, 0.001)

	require.NotNil(t, w.HRData)
	require.NotNil(t, w.HRData.Avg)
	require.NotNil(t, w.HRData.WorkoutMaxHR)
	assert.InDelta(t, 145.0, *w.HRData.Avg, 0.001)
	assert.InDelta(t, 172.0, *w.HRData.WorkoutMaxHR, 0.001)

	require.NotNil(t, w.TSS)
	require.NotNil(t, w.TSS.TrainingStressScore)
	require.NotNil(t, w.TSS.CalculationMethod)
	assert.InDelta(t, 62.4, *w.TSS.TrainingStressScore, 0.001)
	assert.Equal(t, "HR", *w.TSS.CalculationMethod)

	require.Len(t, w.TSSList, 2)
	require.NotNil(t, w.TSSList[1].CalculationMethod)
	assert.Equal(t, "POWER", *w.TSSList[1].CalculationMethod)

	require.Len(t, w.Extensions, 2)
	assert.JSONEq(t, `{"type":"FitnessExtension","vo2Max":52.1,"fitnessAge":28}`, string(w.Extensions[0]))
}

func TestGetWorkout_JSONOutputIncludesOptionalMetricsAndExtensions(t *testing.T) {
	fixture := readFixture(t, "workout_detail_extra_fields.json")
	var envelope struct {
		Payload endpoints.RemoteSyncedWorkoutDetail `json:"payload"`
	}
	require.NoError(t, json.Unmarshal(fixture, &envelope))

	var buf bytes.Buffer
	require.NoError(t, output.Render(&buf, envelope.Payload, output.Opts{Format: "json"}))

	var got map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	require.Contains(t, got, "energyConsumption")
	require.Contains(t, got, "hrdata")
	require.Contains(t, got, "tss")
	require.Contains(t, got, "tssList")
	require.Contains(t, got, "extensions")
	assert.JSONEq(t, `321.5`, string(got["energyConsumption"]))
	assert.JSONEq(t, `{"trainingStressScore":62.4,"calculationMethod":"HR","intensityFactor":0.84,"normalizedPower":210,"averageGradeAdjustedPace":4.75}`, string(got["tss"]))
	assert.JSONEq(t, `[{"type":"FitnessExtension","vo2Max":52.1,"fitnessAge":28},{"type":"SummaryExtension","avgPower":205,"peakEpoc":88}]`, string(got["extensions"]))
}

func TestStats_DecodesEnvelope(t *testing.T) {
	var capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"error":null,"payload":{
			"totalDistanceSum":100000.0,
			"totalTimeSum":36000.0,
			"totalEnergyConsumptionSum":50000.0,
			"totalNumberOfWorkoutsSum":42,
			"totalDays":10,
			"allStats":[
				{"activityId":1,"count":20,"distance":50000.0,"duration":18000.0,"energy":25000.0},
				{"activityId":3,"count":22,"distance":50000.0,"duration":18000.0,"energy":25000.0}
			]
		}}`))
	}))
	defer srv.Close()

	client := api.NewClient(srv.URL+"/v1/", "SK", 0)
	ws, err := endpoints.Stats(context.Background(), client, "alice")
	require.NoError(t, err)
	require.NotNil(t, ws)
	assert.Equal(t, 42, ws.TotalNumberOfWorkoutsSum)
	assert.Len(t, ws.AllStats, 2)
	assert.True(t, strings.Contains(capturedPath, "/workouts/alice/stats"),
		"request URL should contain /workouts/alice/stats, got: %s", capturedPath)
}

func TestFetchSML_StreamsBody(t *testing.T) {
	// 256 bytes of fake JSON-ish payload.
	fakeBody := make([]byte, 256)
	for i := range fakeBody {
		fakeBody[i] = byte(i % 128) // printable-ish bytes, not gzip magic
	}
	// Ensure not accidentally gzip magic (0x1f 0x8b).
	fakeBody[0] = '{'
	fakeBody[1] = '"'

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/workouts/wk1/sml", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(fakeBody)
	}))
	defer srv.Close()

	client := api.NewClient(srv.URL+"/v1/", "SK", 0)
	rc, err := endpoints.FetchSML(context.Background(), client, "wk1")
	require.NoError(t, err)
	require.NotNil(t, rc)
	defer rc.Close()

	got, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, fakeBody, got, "streamed bytes must be byte-identical to server response")
}

func TestFetchFIT_UsesSingularWorkoutPath(t *testing.T) {
	// 32 bytes of fake binary content (not gzip magic).
	fakeBody := []byte("FIT\x00fake-binary-fit-data-padding-xx")
	var capturedPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(fakeBody)
	}))
	defer srv.Close()

	client := api.NewClient(srv.URL+"/v1/", "SK", 0)
	rc, err := endpoints.FetchFIT(context.Background(), client, "wk1")
	require.NoError(t, err)
	require.NotNil(t, rc)
	defer rc.Close()

	// The path MUST be singular "workout/" not "workouts/".
	assert.Equal(t, "/v1/workout/exportFit/wk1", capturedPath,
		"FIT export must use singular /workout/exportFit/{key} path")

	got, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, fakeBody, got, "streamed bytes must be byte-identical to server response")
}

func TestCountWorkouts_RequiresBothParams(t *testing.T) {
	var capturedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"error":null,"payload":{"count":42,"totalCount":100}}`))
	}))
	defer srv.Close()

	client := api.NewClient(srv.URL+"/v1/", "SK", 0)
	wc, err := endpoints.CountWorkouts(context.Background(), client, 1700000000000, 0)
	require.NoError(t, err)
	require.NotNil(t, wc)
	assert.Equal(t, 42, wc.Count)
	assert.Equal(t, 100, wc.TotalCount)

	// Both params must appear in the request URL.
	assert.True(t, strings.Contains(capturedQuery, "until="), "query should contain until= param, got: %s", capturedQuery)
	assert.True(t, strings.Contains(capturedQuery, "sharingFlags="), "query should contain sharingFlags= param, got: %s", capturedQuery)
}

func TestDeleteWorkout_TrailingDeletePath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "DELETE", r.Method)
		require.Equal(t, "/v1/workouts/wk1/delete", r.URL.Path)
		_, _ = w.Write([]byte(`{"error":null,"payload":true}`))
	}))
	defer srv.Close()
	c := api.NewClient(srv.URL+"/v1/", "SK", 0)
	require.NoError(t, endpoints.DeleteWorkout(context.Background(), c, "wk1"))
}

func TestWorkoutList_Summary(t *testing.T) {
	l := endpoints.WorkoutList{Items: []endpoints.RemoteSyncedWorkout{
		{Key: "a", ActivityID: 1, TotalDistance: 5000, TotalTime: 1800, TotalAscent: 50, TotalDescent: 40},
		{Key: "b", ActivityID: 1, TotalDistance: 3000, TotalTime: 1200, TotalAscent: 20, TotalDescent: 15},
		{Key: "c", ActivityID: 5, TotalDistance: 10000, TotalTime: 3600, TotalAscent: 200, TotalDescent: 180},
	}}
	s := l.Summary()
	assert.Equal(t, 3, s.Count)
	assert.InDelta(t, 18000.0, s.TotalDistance, 0.001)
	assert.InDelta(t, 6600.0, s.TotalTime, 0.001)
	assert.InDelta(t, 270.0, s.TotalAscent, 0.001)
	assert.InDelta(t, 235.0, s.TotalDescent, 0.001)
	require.Len(t, s.ByActivity, 2)
	assert.Equal(t, 2, s.ByActivity[1].Count)
	assert.InDelta(t, 8000.0, s.ByActivity[1].Distance, 0.001)
	assert.Equal(t, 1, s.ByActivity[5].Count)

	pretty := s.Pretty()
	assert.Contains(t, pretty, "workouts:  3")
	assert.Contains(t, pretty, "18.00km")
	assert.Contains(t, pretty, "1:50:00")
	assert.Contains(t, pretty, "Per activity:")

	listPretty := l.Pretty()
	assert.Contains(t, listPretty, "3 workouts  18.00km  1:50:00")
}

func TestWorkoutList_Summary_Empty(t *testing.T) {
	l := endpoints.WorkoutList{}
	s := l.Summary()
	assert.Equal(t, 0, s.Count)
	assert.Nil(t, s.ByActivity)
	assert.Contains(t, s.Pretty(), "workouts:  0")
}

func TestWorkoutList_SummaryWithWoW(t *testing.T) {
	now := int64(1_700_000_000_000) // fixed clock for the test
	day := int64(24 * 3600 * 1000)
	l := endpoints.WorkoutList{Items: []endpoints.RemoteSyncedWorkout{
		// this-week bucket: 3 of activity 1, 1 of activity 5
		{Key: "a", ActivityID: 1, StartTime: now - 1*day},
		{Key: "b", ActivityID: 1, StartTime: now - 2*day},
		{Key: "c", ActivityID: 1, StartTime: now - 6*day},
		{Key: "d", ActivityID: 5, StartTime: now - 3*day},
		// prev-week bucket: 1 of activity 1, 3 of activity 5
		{Key: "e", ActivityID: 1, StartTime: now - 9*day},
		{Key: "f", ActivityID: 5, StartTime: now - 8*day},
		{Key: "g", ActivityID: 5, StartTime: now - 10*day},
		{Key: "h", ActivityID: 5, StartTime: now - 13*day},
		// older — ignored for WoW
		{Key: "i", ActivityID: 1, StartTime: now - 30*day},
	}}
	s := l.SummaryWithWoW(now)
	require.NotNil(t, s.WeekOverWeek)
	assert.Equal(t, 2, s.WeekOverWeek[1].Count)  // 3 - 1
	assert.Equal(t, -2, s.WeekOverWeek[5].Count) // 1 - 3

	// Pretty without color includes signed deltas.
	plain := s.Pretty()
	assert.Contains(t, plain, "ΔWoW")
	assert.Contains(t, plain, "+2")
	assert.Contains(t, plain, "-2")

	// With colorizer the values are wrapped — width is unchanged.
	colored := s.RenderPretty(func(p, kind string) string { return "[" + kind + ":" + p + "]" })
	assert.Contains(t, colored, "[pos:+2]")
	assert.Contains(t, colored, "[neg:-2]")
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)
	return b
}
