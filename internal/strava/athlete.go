package strava

import (
	"context"
	"fmt"
	"net/http"

	"github.com/mariiatuzovska/vo2-bot/internal/store/queries"
)

type apiAthlete struct {
	ID        int64    `json:"id"`
	Username  string   `json:"username"`
	Firstname string   `json:"firstname"`
	Lastname  string   `json:"lastname"`
	City      string   `json:"city"`
	Country   string   `json:"country"`
	Sex       string   `json:"sex"`
	Weight    float64  `json:"weight"`
	FTP       *int     `json:"ftp"`
	Profile   string   `json:"profile"`
}

// GetAthlete fetches the athlete profile from Strava and upserts it in the DB.
func (c *Client) GetAthlete(ctx context.Context, athleteID int64) (*apiAthlete, error) {
	resp, err := c.do(ctx, athleteID, "/athlete")
	if err != nil {
		return nil, err
	}
	var a apiAthlete
	if err := decodeJSON(resp, &a); err != nil {
		return nil, err
	}
	if err := c.q.UpsertStravaAthlete(ctx, athleteParams(&a)); err != nil {
		return nil, fmt.Errorf("upsert athlete: %w", err)
	}
	return &a, nil
}

// fetchAthleteWithToken fetches the full athlete profile using a raw access token,
// used during the OAuth callback before tokens are stored in DB.
func (c *Client) fetchAthleteWithToken(ctx context.Context, accessToken string) (*apiAthlete, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/athlete", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	var a apiAthlete
	if err := decodeJSON(resp, &a); err != nil {
		return nil, err
	}
	return &a, nil
}

func athleteParams(a *apiAthlete) queries.UpsertStravaAthleteParams {
	p := queries.UpsertStravaAthleteParams{StravaAthleteID: a.ID}
	setStr := func(dst **string, s string) {
		if s != "" {
			p := s
			*dst = &p
		}
	}
	setStr(&p.Username, a.Username)
	setStr(&p.Firstname, a.Firstname)
	setStr(&p.Lastname, a.Lastname)
	setStr(&p.City, a.City)
	setStr(&p.Country, a.Country)
	setStr(&p.Sex, a.Sex)
	setStr(&p.ProfileUrl, a.Profile)
	if a.Weight > 0 {
		w := a.Weight
		p.WeightKg = &w
	}
	if a.FTP != nil {
		ftp := int32(*a.FTP)
		p.FtpWatts = &ftp
	}
	return p
}
