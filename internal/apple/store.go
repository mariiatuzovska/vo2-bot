package apple

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mariiatuzovska/vo2-bot/internal/store/queries"
)

type Store struct {
	Pool *pgxpool.Pool
	q    *queries.Queries
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{Pool: pool, q: queries.New(pool)}
}

type ImportRecord struct {
	ID             int64
	SourceFilename string
	Sha256         string
	RangeStart     *time.Time
	RangeEnd       *time.Time
	WorkoutsAdded  int
	MetricsAdded   int
	ImportedAt     time.Time
}

func (s *Store) LookupImport(ctx context.Context, sha256 string) (*ImportRecord, error) {
	row, err := s.q.GetImportBySha256(ctx, sha256)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return fromAppleImport(row), nil
}

type persistInput struct {
	sourceFilename string
	sha256         string
	workouts       []typedWorkout
	metrics        []Metric
}

type typedWorkout struct {
	parsed  Workout
	payload json.RawMessage
}

func (s *Store) Persist(ctx context.Context, in persistInput) (*ImportRecord, error) {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	qtx := s.q.WithTx(tx)

	var rangeStart, rangeEnd *time.Time

	for _, tw := range in.workouts {
		w := tw.parsed
		if w.ID == "" {
			return nil, fmt.Errorf("workout missing id")
		}

		if rangeStart == nil || w.Start.Before(*rangeStart) {
			t := w.Start.Time
			rangeStart = &t
		}
		if rangeEnd == nil || w.End.After(*rangeEnd) {
			t := w.End.Time
			rangeEnd = &t
		}

		if err := qtx.UpsertWorkout(ctx, queries.UpsertWorkoutParams{
			ID:               w.ID,
			Name:             w.Name,
			Source:           firstSource(w.HeartRateData),
			IsIndoor:         w.IsIndoor,
			Location:         nullIfEmpty(w.Location),
			StartedAt:        w.Start.Time,
			EndedAt:          w.End.Time,
			DurationSeconds:  w.Duration,
			DistanceKm:       qty(w.Distance),
			ActiveEnergyKj:   qty(w.ActiveEnergyBurned),
			AvgHrBpm:         qty(w.AvgHeartRate),
			MaxHrBpm:         qty(w.MaxHeartRate),
			MinHrBpm:         minHR(w.HeartRate),
			ElevationUpM:     qty(w.ElevationUp),
			AvgSpeed:         qty(w.Speed),
			SpeedUnits:       units(w.Speed),
			StepCadence:      qty(w.StepCadence),
			HumidityPct:      qty(w.Humidity),
			Temperature:      qty(w.Temperature),
			TemperatureUnits: units(w.Temperature),
			Intensity:        qty(w.Intensity),
			Payload:          []byte(tw.payload),
		}); err != nil {
			return nil, fmt.Errorf("upsert workout %s: %w", w.ID, err)
		}

		if err := qtx.DeleteWorkoutHeartRate(ctx, w.ID); err != nil {
			return nil, err
		}
		if err := qtx.DeleteWorkoutRoute(ctx, w.ID); err != nil {
			return nil, err
		}

		if len(w.HeartRateData) > 0 {
			rows := make([][]any, len(w.HeartRateData))
			for i, b := range w.HeartRateData {
				rows[i] = []any{w.ID, b.Date.Time, b.Min, b.Max, b.Avg}
			}
			if _, err := tx.CopyFrom(ctx,
				pgx.Identifier{"apple_workout_heart_rate"},
				[]string{"workout_id", "measured_at", "min_bpm", "max_bpm", "avg_bpm"},
				pgx.CopyFromRows(rows),
			); err != nil {
				return nil, fmt.Errorf("copy heart rate bins: %w", err)
			}
		}

		if len(w.Route) > 0 {
			rows := make([][]any, len(w.Route))
			for i, p := range w.Route {
				rows[i] = []any{
					w.ID, p.Timestamp.Time,
					p.Latitude, p.Longitude,
					p.Altitude, p.Speed,
					p.HorizontalAccuracy, p.CourseAccuracy,
				}
			}
			if _, err := tx.CopyFrom(ctx,
				pgx.Identifier{"apple_workout_route"},
				[]string{"workout_id", "recorded_at", "latitude", "longitude",
					"altitude_m", "speed", "horizontal_accuracy", "course_accuracy"},
				pgx.CopyFromRows(rows),
			); err != nil {
				return nil, fmt.Errorf("copy route: %w", err)
			}
		}
	}

	metricsAdded := 0
	for _, m := range in.metrics {
		for _, p := range m.Data {
			if err := qtx.UpsertDailyMetric(ctx, queries.UpsertDailyMetricParams{
				MetricName: m.Name,
				MeasuredAt: p.Date.Time,
				Source:     p.Source,
				Qty:        p.Qty,
				Units:      m.Units,
			}); err != nil {
				return nil, fmt.Errorf("upsert metric %s: %w", m.Name, err)
			}
			metricsAdded++
		}
	}

	wa := int32(len(in.workouts))
	ma := int32(metricsAdded)
	row, err := qtx.InsertImport(ctx, queries.InsertImportParams{
		SourceFilename: in.sourceFilename,
		SourceSha256:   in.sha256,
		RangeStart:     toTimestamptz(rangeStart),
		RangeEnd:       toTimestamptz(rangeEnd),
		WorkoutsAdded:  &wa,
		MetricsAdded:   &ma,
	})
	if err != nil {
		return nil, fmt.Errorf("insert import record: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return fromAppleImport(row), nil
}

func fromAppleImport(i queries.AppleImport) *ImportRecord {
	rec := &ImportRecord{
		ID:             i.ID,
		SourceFilename: i.SourceFilename,
		Sha256:         i.SourceSha256,
		ImportedAt:     i.ImportedAt,
	}
	if i.RangeStart.Valid {
		t := i.RangeStart.Time
		rec.RangeStart = &t
	}
	if i.RangeEnd.Valid {
		t := i.RangeEnd.Time
		rec.RangeEnd = &t
	}
	if i.WorkoutsAdded != nil {
		rec.WorkoutsAdded = int(*i.WorkoutsAdded)
	}
	if i.MetricsAdded != nil {
		rec.MetricsAdded = int(*i.MetricsAdded)
	}
	return rec
}

func toTimestamptz(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

func qty(q *Quantity) *float64 {
	if q == nil {
		return nil
	}
	v := q.Qty
	return &v
}

func units(q *Quantity) *string {
	if q == nil || q.Units == "" {
		return nil
	}
	v := q.Units
	return &v
}

func minHR(s *HRSummary) *float64 {
	if s == nil {
		return nil
	}
	v := s.Min.Qty
	return &v
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func firstSource(bins []HRBin) *string {
	for _, b := range bins {
		if b.Source != "" {
			s := b.Source
			return &s
		}
	}
	return nil
}
