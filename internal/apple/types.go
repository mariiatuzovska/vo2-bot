package apple

import (
	"fmt"
	"strings"
	"time"
)

// HAE timestamps look like "2026-04-22 21:17:30 -0400" — space-separated,
// local time with zone offset.
const haeTimeLayout = "2006-01-02 15:04:05 -0700"

type Time struct{ time.Time }

func (t *Time) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		return nil
	}
	parsed, err := time.Parse(haeTimeLayout, s)
	if err != nil {
		return fmt.Errorf("hae time %q: %w", s, err)
	}
	t.Time = parsed
	return nil
}

type Quantity struct {
	Qty   float64 `json:"qty"`
	Units string  `json:"units"`
}

type HRSummary struct {
	Min Quantity `json:"min"`
	Max Quantity `json:"max"`
	Avg Quantity `json:"avg"`
}

type HRBin struct {
	Date   Time    `json:"date"`
	Source string  `json:"source"`
	Units  string  `json:"units"`
	Min    float64 `json:"Min"`
	Max    float64 `json:"Max"`
	Avg    float64 `json:"Avg"`
}

type RoutePoint struct {
	Timestamp          Time    `json:"timestamp"`
	Latitude           float64 `json:"latitude"`
	Longitude          float64 `json:"longitude"`
	Altitude           float64 `json:"altitude"`
	Speed              float64 `json:"speed"`
	HorizontalAccuracy float64 `json:"horizontalAccuracy"`
	CourseAccuracy     float64 `json:"courseAccuracy"`
}

type Workout struct {
	ID                 string       `json:"id"`
	Name               string       `json:"name"`
	Start              Time         `json:"start"`
	End                Time         `json:"end"`
	Duration           float64      `json:"duration"`
	IsIndoor           *bool        `json:"isIndoor"`
	Location           string       `json:"location"`
	Distance           *Quantity    `json:"distance"`
	ActiveEnergyBurned *Quantity    `json:"activeEnergyBurned"`
	AvgHeartRate       *Quantity    `json:"avgHeartRate"`
	MaxHeartRate       *Quantity    `json:"maxHeartRate"`
	HeartRate          *HRSummary   `json:"heartRate"`
	Intensity          *Quantity    `json:"intensity"`
	Speed              *Quantity    `json:"speed"`
	StepCadence        *Quantity    `json:"stepCadence"`
	ElevationUp        *Quantity    `json:"elevationUp"`
	Humidity           *Quantity    `json:"humidity"`
	Temperature        *Quantity    `json:"temperature"`
	HeartRateData      []HRBin      `json:"heartRateData"`
	Route              []RoutePoint `json:"route"`
}

type MetricPoint struct {
	Date   Time    `json:"date"`
	Qty    float64 `json:"qty"`
	Source string  `json:"source"`
}

type Metric struct {
	Name  string        `json:"name"`
	Units string        `json:"units"`
	Data  []MetricPoint `json:"data"`
}
