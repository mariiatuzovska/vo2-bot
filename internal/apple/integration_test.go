//go:build integration

package apple

import (
	"context"
	stderrors "errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	vo2bot "github.com/mariiatuzovska/vo2-bot"
	"github.com/mariiatuzovska/vo2-bot/internal/errs"
	"github.com/mariiatuzovska/vo2-bot/internal/store"
)

// fixtureHAEJSON exercises every persistence path: typed workout fields,
// per-minute HR bins, GPS route, and two daily metrics with a source.
const fixtureHAEJSON = `{
  "data": {
    "workouts": [
      {
        "id": "7A71FBAD-1111-2222-3333-444455556666",
        "name": "Outdoor Run",
        "start": "2026-04-22 21:17:30 -0400",
        "end":   "2026-04-22 21:40:06 -0400",
        "duration": 1356,
        "isIndoor": false,
        "location": "Outdoor",
        "distance":      { "qty": 4.21, "units": "km" },
        "activeEnergyBurned": { "qty": 612.3, "units": "kJ" },
        "avgHeartRate":  { "qty": 155.9, "units": "bpm" },
        "maxHeartRate":  { "qty": 163,   "units": "bpm" },
        "heartRate":     {
          "min": {"qty":116,"units":"bpm"},
          "max": {"qty":163,"units":"bpm"},
          "avg": {"qty":155.9,"units":"bpm"}
        },
        "elevationUp":   { "qty": 18.4, "units": "m" },
        "speed":         { "qty": 3.1,  "units": "m/s" },
        "heartRateData": [
          { "date": "2026-04-22 21:18:00 -0400", "source":"Apple Watch", "units":"bpm", "Min": 116, "Max": 147, "Avg": 134.2 },
          { "date": "2026-04-22 21:19:00 -0400", "source":"Apple Watch", "units":"bpm", "Min": 130, "Max": 155, "Avg": 142.0 }
        ],
        "route": [
          { "timestamp":"2026-04-22 21:18:00 -0400", "latitude":48.1, "longitude":16.3, "altitude":180, "speed":3.1, "horizontalAccuracy":5, "courseAccuracy":3 },
          { "timestamp":"2026-04-22 21:19:00 -0400", "latitude":48.11,"longitude":16.31,"altitude":182, "speed":3.2, "horizontalAccuracy":5, "courseAccuracy":3 }
        ]
      },
      {
        "id": "BBBBBBBB-2222-2222-2222-222222222222",
        "name": "Indoor Run",
        "start": "2026-04-22 07:00:00 -0400",
        "end":   "2026-04-22 07:30:00 -0400",
        "duration": 1800,
        "isIndoor": true,
        "distance":     { "qty": 5.0, "units": "km" },
        "avgHeartRate": { "qty": 148, "units": "bpm" }
      }
    ],
    "metrics": [
      {
        "name":  "vo2_max",
        "units": "ml/(kg·min)",
        "data":  [
          { "date": "2026-04-22 00:00:00 -0400", "qty": 47.2, "source": "Apple Watch" },
          { "date": "2026-04-21 00:00:00 -0400", "qty": 47.0, "source": "Apple Watch" }
        ]
      },
      {
        "name":  "heart_rate_variability",
        "units": "ms",
        "data":  [
          { "date": "2026-04-22 00:00:00 -0400", "qty": 65.1, "source": "Apple Watch" }
        ]
      }
    ]
  }
}`

// Shared connection pool: migrations run once, every test gets its own
// transaction so writes never escape the test boundary.
var (
	sharedPoolOnce sync.Once
	sharedPool     *pgxpool.Pool
	sharedPoolErr  error
	sharedPoolDSN  string
)

func sharedDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	sharedPoolOnce.Do(func() {
		dsn := os.Getenv("TEST_DATABASE_URL")
		if dsn == "" {
			dsn = os.Getenv("DATABASE_URL")
		}
		if dsn == "" {
			return
		}
		sharedPoolDSN = dsn

		if err := store.Migrate(dsn, vo2bot.MigrationsFS); err != nil {
			sharedPoolErr = fmt.Errorf("migrate: %w", err)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		pool, err := pgxpool.New(ctx, dsn)
		if err != nil {
			sharedPoolErr = fmt.Errorf("pool: %w", err)
			return
		}

		// Clear any rows left over from previous (non-tx) runs so sha256
		// dedupe doesn't conflict with the fixture archive.
		if _, err := pool.Exec(ctx, `
			TRUNCATE apple_workouts, apple_workout_heart_rate, apple_workout_route,
			         apple_daily_metrics, apple_imports
			RESTART IDENTITY CASCADE
		`); err != nil {
			pool.Close()
			sharedPoolErr = fmt.Errorf("truncate: %w", err)
			return
		}

		sharedPool = pool
	})

	if sharedPoolErr != nil {
		t.Fatalf("shared db: %v", sharedPoolErr)
	}
	if sharedPool == nil {
		t.Skip("set TEST_DATABASE_URL or DATABASE_URL to run integration tests")
	}
	return sharedPool
}

// newTxService starts a tx on the shared pool and returns a Service whose
// writes are scoped to that tx. The tx is rolled back when the test ends,
// so each test sees an empty schema and never sees writes from other tests.
func newTxService(t *testing.T) (*Service, string) {
	t.Helper()

	pool := sharedDB(t)
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	t.Cleanup(func() {
		_ = tx.Rollback(context.Background())
	})

	tmp := t.TempDir()

	return &Service{
		Source: &LocalSource{BaseDir: tmp},
		Store:  NewStore(tx),
	}, tmp
}

func writeFixtureZip(t *testing.T, dir, name, jsonBody string) {
	t.Helper()
	zipBytes := makeArchive(t, map[string]string{
		"HealthAutoExport-2026-04.json": jsonBody,
	})
	if err := os.WriteFile(filepath.Join(dir, name), zipBytes, 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
}

func TestImport_RoundTrip(t *testing.T) {
	svc, dir := newTxService(t)
	const archiveName = "HealthAutoExport_20260423132853.zip"
	writeFixtureZip(t, dir, archiveName, fixtureHAEJSON)

	ctx := context.Background()
	res, err := svc.Import(ctx, ImportRequest{Source: "local", Name: archiveName})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if res.WorkoutsAdded != 2 {
		t.Errorf("WorkoutsAdded=%d, want 2", res.WorkoutsAdded)
	}
	if res.MetricsAdded != 3 {
		t.Errorf("MetricsAdded=%d, want 3", res.MetricsAdded)
	}
	if res.Sha256 == "" {
		t.Errorf("Sha256 empty")
	}
	if res.RangeStart == nil || res.RangeEnd == nil {
		t.Errorf("range bounds nil: start=%v end=%v", res.RangeStart, res.RangeEnd)
	}

	from, _ := time.Parse(time.RFC3339, "2026-04-22T00:00:00-04:00")
	to, _ := time.Parse(time.RFC3339, "2026-04-23T00:00:00-04:00")

	t.Run("workouts without children", func(t *testing.T) {
		w, err := svc.Workouts(ctx, WorkoutsRequest{From: from, To: to})
		if err != nil {
			t.Fatalf("workouts: %v", err)
		}
		if len(w.Workouts) != 2 {
			t.Fatalf("got %d workouts, want 2", len(w.Workouts))
		}
		// Ordered started_at DESC: outdoor (21:17) comes before indoor (07:00).
		if w.Workouts[0].Name != "Outdoor Run" || w.Workouts[1].Name != "Indoor Run" {
			t.Errorf("ordering: %q, %q", w.Workouts[0].Name, w.Workouts[1].Name)
		}
		if w.Workouts[0].HeartRate != nil {
			t.Errorf("HR should be omitted when include_hr=false")
		}
		if w.Workouts[0].Route != nil {
			t.Errorf("Route should be omitted when include_route=false")
		}
	})

	t.Run("workouts with HR + route", func(t *testing.T) {
		w, err := svc.Workouts(ctx, WorkoutsRequest{
			From: from, To: to,
			Names:        []string{"Outdoor Run"},
			IncludeHR:    true,
			IncludeRoute: true,
		})
		if err != nil {
			t.Fatalf("workouts: %v", err)
		}
		if len(w.Workouts) != 1 {
			t.Fatalf("got %d workouts, want 1", len(w.Workouts))
		}
		got := w.Workouts[0]
		if len(got.HeartRate) != 2 {
			t.Errorf("HR bins=%d, want 2", len(got.HeartRate))
		}
		if len(got.Route) != 2 {
			t.Errorf("route points=%d, want 2", len(got.Route))
		}
		if got.DistanceKm == nil || *got.DistanceKm != 4.21 {
			t.Errorf("distance: %v", got.DistanceKm)
		}
	})

	t.Run("workout name filter excludes others", func(t *testing.T) {
		w, err := svc.Workouts(ctx, WorkoutsRequest{
			From: from, To: to,
			Names: []string{"Pool Swim"},
		})
		if err != nil {
			t.Fatalf("workouts: %v", err)
		}
		if len(w.Workouts) != 0 {
			t.Errorf("expected empty, got %d", len(w.Workouts))
		}
	})

	t.Run("metrics", func(t *testing.T) {
		fromAll, _ := time.Parse(time.RFC3339, "2026-04-21T00:00:00-04:00")
		m, err := svc.Metrics(ctx, MetricsRequest{From: fromAll, To: to})
		if err != nil {
			t.Fatalf("metrics: %v", err)
		}
		if got := len(m.Metrics["vo2_max"]); got != 2 {
			t.Errorf("vo2_max points=%d, want 2", got)
		}
		if got := len(m.Metrics["heart_rate_variability"]); got != 1 {
			t.Errorf("HRV points=%d, want 1", got)
		}
	})

	t.Run("metrics name filter", func(t *testing.T) {
		fromAll, _ := time.Parse(time.RFC3339, "2026-04-21T00:00:00-04:00")
		m, err := svc.Metrics(ctx, MetricsRequest{
			From: fromAll, To: to,
			Names: []string{"vo2_max"},
		})
		if err != nil {
			t.Fatalf("metrics: %v", err)
		}
		if _, ok := m.Metrics["heart_rate_variability"]; ok {
			t.Errorf("HRV should be filtered out")
		}
		if got := len(m.Metrics["vo2_max"]); got != 2 {
			t.Errorf("vo2_max points=%d, want 2", got)
		}
	})

	t.Run("catalog", func(t *testing.T) {
		c, err := svc.Catalog(ctx)
		if err != nil {
			t.Fatalf("catalog: %v", err)
		}
		wantWorkouts := map[string]bool{"Outdoor Run": true, "Indoor Run": true}
		for _, n := range c.WorkoutNames {
			delete(wantWorkouts, n)
		}
		if len(wantWorkouts) != 0 {
			t.Errorf("missing workout names: %v (got %v)", wantWorkouts, c.WorkoutNames)
		}
		wantMetrics := map[string]bool{"vo2_max": true, "heart_rate_variability": true}
		for _, n := range c.MetricNames {
			delete(wantMetrics, n)
		}
		if len(wantMetrics) != 0 {
			t.Errorf("missing metric names: %v (got %v)", wantMetrics, c.MetricNames)
		}
	})
}

func TestImport_DuplicateReturns409(t *testing.T) {
	svc, dir := newTxService(t)
	const archiveName = "HealthAutoExport_dup.zip"
	writeFixtureZip(t, dir, archiveName, fixtureHAEJSON)

	ctx := context.Background()
	if _, err := svc.Import(ctx, ImportRequest{Source: "local", Name: archiveName}); err != nil {
		t.Fatalf("first import: %v", err)
	}

	_, err := svc.Import(ctx, ImportRequest{Source: "local", Name: archiveName})
	if err == nil {
		t.Fatal("expected conflict error, got nil")
	}
	var apiErr *errs.Error
	if !stderrors.As(err, &apiErr) || apiErr.Status != http.StatusConflict {
		t.Fatalf("expected 409 errs.Error, got %T %v", err, err)
	}
}
