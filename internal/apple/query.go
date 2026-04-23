package apple

import (
	"context"
	"fmt"
	"time"

	"github.com/mariiatuzovska/vo2-bot/internal/store/queries"
)

const (
	defaultWorkoutLimit = 100
	maxWorkoutLimit     = 1000
)

type WorkoutsRequest struct {
	From         time.Time
	To           time.Time
	Names        []string
	IncludeHR    bool
	IncludeRoute bool
	Limit        int32
}

type WorkoutsResponse struct {
	Range    InstantRange  `json:"range"`
	Workouts []WorkoutView `json:"workouts"`
}

// InstantRange is the half-open window [From, To) echoed back to the caller.
type InstantRange struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

type WorkoutView struct {
	ID               string           `json:"id"`
	Name             string           `json:"name"`
	Source           *string          `json:"source,omitempty"`
	IsIndoor         *bool            `json:"is_indoor,omitempty"`
	Location         *string          `json:"location,omitempty"`
	StartedAt        time.Time        `json:"started_at"`
	EndedAt          time.Time        `json:"ended_at"`
	DurationSeconds  float64          `json:"duration_seconds"`
	DistanceKm       *float64         `json:"distance_km,omitempty"`
	ActiveEnergyKj   *float64         `json:"active_energy_kj,omitempty"`
	AvgHrBpm         *float64         `json:"avg_hr_bpm,omitempty"`
	MaxHrBpm         *float64         `json:"max_hr_bpm,omitempty"`
	MinHrBpm         *float64         `json:"min_hr_bpm,omitempty"`
	ElevationUpM     *float64         `json:"elevation_up_m,omitempty"`
	AvgSpeed         *float64         `json:"avg_speed,omitempty"`
	SpeedUnits       *string          `json:"speed_units,omitempty"`
	StepCadence      *float64         `json:"step_cadence,omitempty"`
	HumidityPct      *float64         `json:"humidity_pct,omitempty"`
	Temperature      *float64         `json:"temperature,omitempty"`
	TemperatureUnits *string          `json:"temperature_units,omitempty"`
	Intensity        *float64         `json:"intensity,omitempty"`
	HeartRate        []HRBinView      `json:"heart_rate,omitempty"`
	Route            []RoutePointView `json:"route,omitempty"`
}

type HRBinView struct {
	MeasuredAt time.Time `json:"measured_at"`
	Min        *float64  `json:"min,omitempty"`
	Max        *float64  `json:"max,omitempty"`
	Avg        *float64  `json:"avg,omitempty"`
}

type RoutePointView struct {
	RecordedAt         time.Time `json:"recorded_at"`
	Latitude           float64   `json:"lat"`
	Longitude          float64   `json:"lon"`
	AltitudeM          *float64  `json:"altitude_m,omitempty"`
	Speed              *float64  `json:"speed,omitempty"`
	HorizontalAccuracy *float64  `json:"horizontal_accuracy,omitempty"`
	CourseAccuracy    *float64   `json:"course_accuracy,omitempty"`
}

type MetricsRequest struct {
	From    time.Time
	To      time.Time
	Names   []string
	Sources []string
}

type MetricsResponse struct {
	Range   InstantRange                   `json:"range"`
	Metrics map[string][]MetricSeriesPoint `json:"metrics"`
}

type MetricSeriesPoint struct {
	Date   time.Time `json:"date"`
	Qty    float64   `json:"qty"`
	Units  string    `json:"units"`
	Source string    `json:"source"`
}

type CatalogResponse struct {
	WorkoutNames []string `json:"workout_names"`
	MetricNames  []string `json:"metric_names"`
}

func (s *Service) Workouts(ctx context.Context, req WorkoutsRequest) (*WorkoutsResponse, error) {
	limit := req.Limit
	if limit <= 0 {
		limit = defaultWorkoutLimit
	}
	if limit > maxWorkoutLimit {
		limit = maxWorkoutLimit
	}

	rows, err := s.Store.ListWorkouts(ctx, queries.ListWorkoutsParams{
		FromAt: req.From,
		ToAt:   req.To,
		Names:  req.Names,
		Lim:    limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list workouts: %w", err)
	}

	views := make([]WorkoutView, len(rows))
	ids := make([]string, len(rows))
	idx := make(map[string]int, len(rows))
	for i, r := range rows {
		views[i] = WorkoutView{
			ID:               r.ID,
			Name:             r.Name,
			Source:           r.Source,
			IsIndoor:         r.IsIndoor,
			Location:         r.Location,
			StartedAt:        r.StartedAt,
			EndedAt:          r.EndedAt,
			DurationSeconds:  r.DurationSeconds,
			DistanceKm:       r.DistanceKm,
			ActiveEnergyKj:   r.ActiveEnergyKj,
			AvgHrBpm:         r.AvgHrBpm,
			MaxHrBpm:         r.MaxHrBpm,
			MinHrBpm:         r.MinHrBpm,
			ElevationUpM:     r.ElevationUpM,
			AvgSpeed:         r.AvgSpeed,
			SpeedUnits:       r.SpeedUnits,
			StepCadence:      r.StepCadence,
			HumidityPct:      r.HumidityPct,
			Temperature:      r.Temperature,
			TemperatureUnits: r.TemperatureUnits,
			Intensity:        r.Intensity,
		}
		ids[i] = r.ID
		idx[r.ID] = i
	}

	if req.IncludeHR && len(ids) > 0 {
		bins, err := s.Store.ListWorkoutHeartRate(ctx, ids)
		if err != nil {
			return nil, fmt.Errorf("list workout hr: %w", err)
		}
		for _, b := range bins {
			i, ok := idx[b.WorkoutID]
			if !ok {
				continue
			}
			views[i].HeartRate = append(views[i].HeartRate, HRBinView{
				MeasuredAt: b.MeasuredAt,
				Min:        b.MinBpm,
				Max:        b.MaxBpm,
				Avg:        b.AvgBpm,
			})
		}
	}

	if req.IncludeRoute && len(ids) > 0 {
		pts, err := s.Store.ListWorkoutRoute(ctx, ids)
		if err != nil {
			return nil, fmt.Errorf("list workout route: %w", err)
		}
		for _, p := range pts {
			i, ok := idx[p.WorkoutID]
			if !ok {
				continue
			}
			views[i].Route = append(views[i].Route, RoutePointView{
				RecordedAt:         p.RecordedAt,
				Latitude:           p.Latitude,
				Longitude:          p.Longitude,
				AltitudeM:          p.AltitudeM,
				Speed:              p.Speed,
				HorizontalAccuracy: p.HorizontalAccuracy,
				CourseAccuracy:     p.CourseAccuracy,
			})
		}
	}

	return &WorkoutsResponse{
		Range:    InstantRange{From: req.From, To: req.To},
		Workouts: views,
	}, nil
}

func (s *Service) Metrics(ctx context.Context, req MetricsRequest) (*MetricsResponse, error) {
	rows, err := s.Store.ListDailyMetrics(ctx, queries.ListDailyMetricsParams{
		FromAt:  req.From,
		ToAt:    req.To,
		Names:   req.Names,
		Sources: req.Sources,
	})
	if err != nil {
		return nil, fmt.Errorf("list metrics: %w", err)
	}

	out := make(map[string][]MetricSeriesPoint)
	for _, r := range rows {
		out[r.MetricName] = append(out[r.MetricName], MetricSeriesPoint{
			Date:   r.MeasuredAt,
			Qty:    r.Qty,
			Units:  r.Units,
			Source: r.Source,
		})
	}

	return &MetricsResponse{
		Range:   InstantRange{From: req.From, To: req.To},
		Metrics: out,
	}, nil
}

func (s *Service) Catalog(ctx context.Context) (*CatalogResponse, error) {
	workouts, err := s.Store.ListWorkoutNames(ctx)
	if err != nil {
		return nil, fmt.Errorf("list workout names: %w", err)
	}
	metrics, err := s.Store.ListMetricNames(ctx)
	if err != nil {
		return nil, fmt.Errorf("list metric names: %w", err)
	}
	return &CatalogResponse{WorkoutNames: workouts, MetricNames: metrics}, nil
}

