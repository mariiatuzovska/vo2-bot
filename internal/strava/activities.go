package strava

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type apiActivity struct {
	ID                 int64     `json:"id"`
	Name               string    `json:"name"`
	SportType          string    `json:"sport_type"`
	Type               string    `json:"type"` // legacy fallback
	WorkoutType        *int      `json:"workout_type"`
	StartDate          time.Time `json:"start_date"`
	StartDateLocal     time.Time `json:"start_date_local"`
	Timezone           string    `json:"timezone"`
	Distance           float64   `json:"distance"`
	MovingTime         int       `json:"moving_time"`
	ElapsedTime        int       `json:"elapsed_time"`
	TotalElevationGain float64   `json:"total_elevation_gain"`
	AverageSpeed       float64   `json:"average_speed"`
	MaxSpeed           float64   `json:"max_speed"`
	AverageHeartrate   *float64  `json:"average_heartrate"`
	MaxHeartrate       *float64  `json:"max_heartrate"`
	AverageWatts       *float64  `json:"average_watts"`
	AverageCadence     *float64  `json:"average_cadence"`
	SufferScore        *int      `json:"suffer_score"`
	Trainer            bool      `json:"trainer"`
	Commute            bool      `json:"commute"`
}

func (a *apiActivity) sportType() string {
	if a.SportType != "" {
		return a.SportType
	}
	return a.Type
}

const activitiesPerPage = 200

// listPage fetches one page of activities. after is a Unix timestamp (0 = all history).
func (c *Client) listPage(ctx context.Context, athleteID, after int64, page int) ([]apiActivity, error) {
	path := fmt.Sprintf("/athlete/activities?per_page=%d&page=%d", activitiesPerPage, page)
	if after > 0 {
		path += fmt.Sprintf("&after=%d", after)
	}

	resp, err := c.do(ctx, athleteID, path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var acts []apiActivity
	if err := json.NewDecoder(resp.Body).Decode(&acts); err != nil {
		return nil, fmt.Errorf("decode activities page %d: %w", page, err)
	}
	return acts, nil
}
