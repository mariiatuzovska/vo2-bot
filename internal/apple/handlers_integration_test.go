//go:build integration

package apple

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newHandler(t *testing.T) (*Handler, *Service, string) {
	t.Helper()
	svc, dir := newTxService(t)
	return &Handler{Service: svc}, svc, dir
}

func mustImportFixture(t *testing.T, svc *Service, dir, name string) {
	t.Helper()
	writeFixtureZip(t, dir, name, fixtureHAEJSON)
	if _, err := svc.Import(context.Background(), ImportRequest{Source: "local", Name: name}); err != nil {
		t.Fatalf("import: %v", err)
	}
}

func serve(h *Handler, req *http.Request) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	h.Register(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestHandler_Import_Accepted(t *testing.T) {
	h, _, dir := newHandler(t)
	const name = "HealthAutoExport_handler.zip"
	writeFixtureZip(t, dir, name, fixtureHAEJSON)

	body := strings.NewReader(`{"source":"local","name":"` + name + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/apple/import", body)
	rec := serve(h, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var got ImportResult
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.WorkoutsAdded != 2 || got.MetricsAdded != 3 {
		t.Errorf("counts: workouts=%d metrics=%d", got.WorkoutsAdded, got.MetricsAdded)
	}
}

func TestHandler_Import_BadJSON(t *testing.T) {
	h, _, _ := newHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/apple/import", strings.NewReader("{not json"))
	rec := serve(h, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandler_Workouts_OK(t *testing.T) {
	h, svc, dir := newHandler(t)
	mustImportFixture(t, svc, dir, "HealthAutoExport_workouts.zip")

	req := httptest.NewRequest(http.MethodGet,
		"/apple/workouts?from=2026-04-22T00:00:00-04:00&to=2026-04-23T00:00:00-04:00&include_hr=true&include_route=true",
		nil)
	rec := serve(h, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var got WorkoutsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Workouts) != 2 {
		t.Fatalf("workouts=%d, want 2", len(got.Workouts))
	}
	// Outdoor Run is the first row (DESC by started_at) and has HR + GPS.
	if len(got.Workouts[0].HeartRate) == 0 || len(got.Workouts[0].Route) == 0 {
		t.Errorf("expected hr+route on first workout, got hr=%d route=%d",
			len(got.Workouts[0].HeartRate), len(got.Workouts[0].Route))
	}
}

func TestHandler_Workouts_BadRequest(t *testing.T) {
	h, _, _ := newHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/apple/workouts", nil)
	rec := serve(h, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandler_Workouts_BadLimit(t *testing.T) {
	h, _, _ := newHandler(t)
	req := httptest.NewRequest(http.MethodGet,
		"/apple/workouts?from=2026-04-22T00:00:00Z&to=2026-04-23T00:00:00Z&limit=oops",
		nil)
	rec := serve(h, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandler_Metrics_OK(t *testing.T) {
	h, svc, dir := newHandler(t)
	mustImportFixture(t, svc, dir, "HealthAutoExport_metrics.zip")

	req := httptest.NewRequest(http.MethodGet,
		"/apple/metrics?from=2026-04-21T00:00:00-04:00&to=2026-04-23T00:00:00-04:00&name=vo2_max",
		nil)
	rec := serve(h, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var got MetricsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := got.Metrics["heart_rate_variability"]; ok {
		t.Errorf("HRV should be filtered out")
	}
	if len(got.Metrics["vo2_max"]) != 2 {
		t.Errorf("vo2_max points=%d want 2", len(got.Metrics["vo2_max"]))
	}
}

func TestHandler_Metrics_BadRequest(t *testing.T) {
	h, _, _ := newHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/apple/metrics?from=2026-04-21", nil)
	rec := serve(h, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandler_Catalog(t *testing.T) {
	h, svc, dir := newHandler(t)
	mustImportFixture(t, svc, dir, "HealthAutoExport_catalog.zip")

	req := httptest.NewRequest(http.MethodGet, "/apple/catalog", nil)
	rec := serve(h, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var got CatalogResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	wantWorkouts := map[string]bool{"Outdoor Run": true, "Indoor Run": true}
	for _, n := range got.WorkoutNames {
		delete(wantWorkouts, n)
	}
	if len(wantWorkouts) != 0 {
		t.Errorf("missing workout names: %v (got %v)", wantWorkouts, got.WorkoutNames)
	}
	wantMetrics := map[string]bool{"vo2_max": true, "heart_rate_variability": true}
	for _, n := range got.MetricNames {
		delete(wantMetrics, n)
	}
	if len(wantMetrics) != 0 {
		t.Errorf("missing metric names: %v (got %v)", wantMetrics, got.MetricNames)
	}
}

// Imports a deliberately broken workout so per-workout Unmarshal fails after
// LookupImport succeeds — the only path that needs a real Store.
func TestImport_MalformedWorkout_422(t *testing.T) {
	svc, dir := newTxService(t)
	const name = "HealthAutoExport_malformed.zip"
	writeFixtureZip(t, dir, name, `{
	  "data": {
	    "workouts": [
	      { "id":"X", "name":"Bad", "start":"garbage", "end":"garbage", "duration":1 }
	    ],
	    "metrics": []
	  }
	}`)
	_, err := svc.Import(context.Background(), ImportRequest{Source: "local", Name: name})
	assertStatus(t, err, http.StatusUnprocessableEntity)
}
