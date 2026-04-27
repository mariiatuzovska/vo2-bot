package apple

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mariiatuzovska/vo2-bot/internal/errs"
)

type Service struct {
	Source Source
	Store  *Store
}

type ImportRequest struct {
	Source string `json:"source"`
	Name   string `json:"name"`
}

type ImportResult struct {
	Sha256        string         `json:"sha256"`
	RangeStart    *time.Time     `json:"range_start"`
	RangeEnd      *time.Time     `json:"range_end"`
	WorkoutsAdded int            `json:"workouts_added"`
	MetricsAdded  int            `json:"metrics_added"`
	ImportedAt    time.Time      `json:"imported_at"`
	Latest        *LatestWorkout `json:"latest,omitempty"`
}

type LatestWorkout struct {
	Name      string    `json:"name"`
	StartDate time.Time `json:"start_date"`
}

func (s *Service) Import(ctx context.Context, req ImportRequest) (*ImportResult, error) {
	switch req.Source {
	case "local":
		if req.Name == "" {
			latest, err := s.Source.Latest()
			if err != nil {
				return nil, err
			}
			req.Name = latest
		}
	case "gcs":
		return nil, errs.NewBadRequest("source=gcs not implemented yet")
	case "":
		return nil, errs.NewBadRequest("source is required")
	default:
		return nil, errs.NewBadRequest("unknown source: %s", req.Source)
	}

	data, err := s.Source.Read(req.Name)
	if err != nil {
		return nil, err
	}

	parsed, err := extract(data)
	if err != nil {
		return nil, errs.NewUnprocessable("%s", err)
	}

	if existing, err := s.Store.LookupImport(ctx, parsed.sha256); err != nil {
		return nil, fmt.Errorf("lookup import: %w", err)
	} else if existing != nil {
		return nil, errs.NewConflict("").
			With("sha256", existing.Sha256).
			With("imported_at", existing.ImportedAt)
	}

	typed := make([]typedWorkout, 0, len(parsed.workouts))
	var latest *LatestWorkout
	for i, raw := range parsed.workouts {
		var w Workout
		if err := json.Unmarshal(raw, &w); err != nil {
			return nil, errs.NewUnprocessable("workout[%d]: %v", i, err)
		}
		typed = append(typed, typedWorkout{parsed: w, payload: raw})
		if latest == nil || w.Start.Time.After(latest.StartDate) {
			latest = &LatestWorkout{Name: w.Name, StartDate: w.Start.Time}
		}
	}

	rec, err := s.Store.Persist(ctx, persistInput{
		sourceFilename: req.Name,
		sha256:         parsed.sha256,
		workouts:       typed,
		metrics:        parsed.metrics,
	})
	if err != nil {
		return nil, fmt.Errorf("persist: %w", err)
	}

	return &ImportResult{
		Sha256:        rec.Sha256,
		RangeStart:    rec.RangeStart,
		RangeEnd:      rec.RangeEnd,
		WorkoutsAdded: rec.WorkoutsAdded,
		MetricsAdded:  rec.MetricsAdded,
		ImportedAt:    rec.ImportedAt,
		Latest:        latest,
	}, nil
}
