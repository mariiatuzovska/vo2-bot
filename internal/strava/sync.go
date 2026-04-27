package strava

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/mariiatuzovska/vo2-bot/internal/store/queries"
)

type LatestActivity struct {
	Name      string
	SportType string
	StartDate time.Time
}

type SyncResult struct {
	Added  int
	Total  int64
	Latest *LatestActivity
}

// Sync pulls new Strava activities for the athlete linked to chatID.
// An advisory lock prevents concurrent /pull calls for the same athlete.
func (c *Client) Sync(ctx context.Context, chatID int64) (*SyncResult, error) {
	athleteID, err := c.q.ResolveAthleteByChat(ctx, chatID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("not linked — run /login first")
	}
	if err != nil {
		return nil, fmt.Errorf("resolve athlete: %w", err)
	}

	// Acquire a dedicated connection for the session-level advisory lock so
	// the lock is held for the duration of the sync regardless of pool routing.
	conn, err := c.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire conn: %w", err)
	}
	defer conn.Release()

	var locked bool
	if err := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", athleteID).Scan(&locked); err != nil {
		return nil, fmt.Errorf("advisory lock: %w", err)
	}
	if !locked {
		return nil, fmt.Errorf("already syncing — try again shortly")
	}
	defer conn.Exec(ctx, "SELECT pg_advisory_unlock($1)", athleteID) //nolint:errcheck

	// Determine cursor for incremental sync.
	state, stateErr := c.q.GetSyncState(ctx, athleteID)
	if stateErr != nil && !errors.Is(stateErr, pgx.ErrNoRows) {
		return nil, fmt.Errorf("get sync state: %w", stateErr)
	}
	var after int64
	if stateErr == nil && state.LastActivityAt.Valid {
		after = state.LastActivityAt.Time.Unix()
	}

	// Page through all activities since cursor.
	var (
		added      int
		latestTime time.Time
		latest     *LatestActivity
	)
	for page := 1; ; page++ {
		acts, err := c.listPage(ctx, athleteID, after, page)
		if err != nil {
			return nil, err
		}
		if len(acts) == 0 {
			break
		}
		for i := range acts {
			a := &acts[i]
			raw, _ := json.Marshal(a)
			if err := c.q.UpsertStravaActivity(ctx, activityParams(athleteID, a, raw)); err != nil {
				return nil, fmt.Errorf("upsert activity %d: %w", a.ID, err)
			}
			added++
			if a.StartDate.After(latestTime) {
				latestTime = a.StartDate
				latest = &LatestActivity{
					Name:      a.Name,
					SportType: a.sportType(),
					StartDate: a.StartDate,
				}
			}
		}
	}

	// Preserve existing cursor when no new activities found.
	newCursor := pgtype.Timestamptz{}
	if latest != nil {
		newCursor = pgtype.Timestamptz{Time: latestTime, Valid: true}
	} else if stateErr == nil {
		newCursor = state.LastActivityAt
	}

	if err := c.q.UpsertSyncState(ctx, queries.UpsertSyncStateParams{
		StravaAthleteID: athleteID,
		LastActivityAt:  newCursor,
	}); err != nil {
		return nil, fmt.Errorf("update sync state: %w", err)
	}

	total, err := c.q.CountStravaActivities(ctx, athleteID)
	if err != nil {
		return nil, fmt.Errorf("count activities: %w", err)
	}

	return &SyncResult{Added: added, Total: total, Latest: latest}, nil
}

func activityParams(athleteID int64, a *apiActivity, raw []byte) queries.UpsertStravaActivityParams {
	p := queries.UpsertStravaActivityParams{
		StravaActivityID: a.ID,
		StravaAthleteID:  athleteID,
		Name:             a.Name,
		SportType:        a.sportType(),
		StartAt:          a.StartDate,
		Payload:          raw,
	}
	if a.WorkoutType != nil {
		wt := int32(*a.WorkoutType)
		p.WorkoutType = &wt
	}
	if !a.StartDateLocal.IsZero() {
		p.StartAtLocal = pgtype.Timestamptz{Time: a.StartDateLocal, Valid: true}
	}
	if a.Timezone != "" {
		p.Timezone = &a.Timezone
	}
	if a.Distance > 0 {
		p.DistanceM = &a.Distance
	}
	if a.MovingTime > 0 {
		mt := int32(a.MovingTime)
		p.MovingTimeS = &mt
	}
	if a.ElapsedTime > 0 {
		et := int32(a.ElapsedTime)
		p.ElapsedTimeS = &et
	}
	if a.TotalElevationGain > 0 {
		p.ElevationGainM = &a.TotalElevationGain
	}
	if a.AverageSpeed > 0 {
		p.AverageSpeedMps = &a.AverageSpeed
	}
	if a.MaxSpeed > 0 {
		p.MaxSpeedMps = &a.MaxSpeed
	}
	p.AverageHeartrate = a.AverageHeartrate
	p.MaxHeartrate = a.MaxHeartrate
	p.AverageWatts = a.AverageWatts
	p.AverageCadence = a.AverageCadence
	if a.SufferScore != nil {
		ss := int32(*a.SufferScore)
		p.SufferScore = &ss
	}
	p.Trainer = &a.Trainer
	p.Commute = &a.Commute
	return p
}
