package strava

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mariiatuzovska/vo2-bot/internal/store/queries"
)

const (
	baseURL  = "https://www.strava.com/api/v3"
	tokenURL = "https://www.strava.com/oauth/token"
)

var (
	ErrRateLimited  = errors.New("strava: rate limited")
	ErrTokenRevoked = errors.New("strava: token revoked — re-run /login")
)

type Client struct {
	http         *http.Client
	clientID     string
	clientSecret string
	redirectURL  string
	q            *queries.Queries
	pool         *pgxpool.Pool
}

func New(clientID, clientSecret, redirectURL string, pool *pgxpool.Pool) *Client {
	return &Client{
		http:         &http.Client{Timeout: 30 * time.Second},
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURL:  redirectURL,
		q:            queries.New(pool),
		pool:         pool,
	}
}

// do executes an authenticated GET for athleteID, refreshing the token if
// expired, then updates the rate-limit row from response headers.
func (c *Client) do(ctx context.Context, athleteID int64, path string) (*http.Response, error) {
	token, err := c.tokenFor(ctx, athleteID)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}

	c.trackRateLimit(ctx, resp)

	if resp.StatusCode == http.StatusTooManyRequests {
		resp.Body.Close()
		return nil, ErrRateLimited
	}
	return resp, nil
}

func (c *Client) tokenFor(ctx context.Context, athleteID int64) (string, error) {
	tok, err := c.q.GetStravaTokens(ctx, athleteID)
	if err != nil {
		return "", fmt.Errorf("get tokens: %w", err)
	}
	if time.Now().Before(tok.ExpiresAt.Add(-30 * time.Second)) {
		return tok.AccessToken, nil
	}
	return c.refresh(ctx, athleteID, tok.RefreshToken)
}

type refreshResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    int64  `json:"expires_at"`
}

func (c *Client) refresh(ctx context.Context, athleteID int64, refreshToken string) (string, error) {
	vals := url.Values{
		"client_id":     {c.clientID},
		"client_secret": {c.clientSecret},
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL,
		strings.NewReader(vals.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusBadRequest {
		_ = c.q.DeleteStravaTokens(ctx, athleteID)
		return "", ErrTokenRevoked
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("strava token refresh HTTP %d: %s", resp.StatusCode, body)
	}

	var r refreshResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return "", fmt.Errorf("decode refresh response: %w", err)
	}

	if err := c.q.UpsertStravaTokens(ctx, queries.UpsertStravaTokensParams{
		StravaAthleteID: athleteID,
		AccessToken:     r.AccessToken,
		RefreshToken:    r.RefreshToken,
		ExpiresAt:       time.Unix(r.ExpiresAt, 0),
	}); err != nil {
		return "", fmt.Errorf("persist refreshed tokens: %w", err)
	}
	return r.AccessToken, nil
}

// trackRateLimit reads X-RateLimit-Limit / X-RateLimit-Usage headers (format:
// "15min,daily") and updates the strava_rate_limit row. Best-effort: errors ignored.
func (c *Client) trackRateLimit(ctx context.Context, resp *http.Response) {
	lims := strings.SplitN(resp.Header.Get("X-RateLimit-Limit"), ",", 2)
	uses := strings.SplitN(resp.Header.Get("X-RateLimit-Usage"), ",", 2)
	if len(lims) != 2 || len(uses) != 2 {
		return
	}
	parse := func(s string) (int32, bool) {
		n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 32)
		return int32(n), err == nil
	}
	sl, ok1 := parse(lims[0])
	dl, ok2 := parse(lims[1])
	su, ok3 := parse(uses[0])
	du, ok4 := parse(uses[1])
	if !ok1 || !ok2 || !ok3 || !ok4 {
		return
	}
	_ = c.q.UpdateRateLimit(ctx, queries.UpdateRateLimitParams{
		ShortLimit: sl, ShortUsage: su,
		DailyLimit: dl, DailyUsage: du,
	})
}

func decodeJSON(resp *http.Response, dst any) error {
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("strava HTTP %d: %s", resp.StatusCode, body)
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}
