package strava

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/mariiatuzovska/vo2-bot/internal/store/queries"
)

const (
	stravaAuthURL = "https://www.strava.com/oauth/authorize"
	stateTTL      = 10 * time.Minute
)

// AuthURL returns a Strava OAuth2 authorization URL with an HMAC-signed state
// that embeds chatID and a 10-minute expiry.
func (c *Client) AuthURL(chatID int64) string {
	expiry := time.Now().Add(stateTTL).Unix()
	payload := fmt.Sprintf("%d:%d", chatID, expiry)
	state := payload + "." + c.sign(payload)

	params := url.Values{
		"client_id":       {c.clientID},
		"redirect_uri":    {c.redirectURL},
		"response_type":   {"code"},
		"approval_prompt": {"auto"},
		"scope":           {"read,activity:read_all"},
		"state":           {state},
	}
	return stravaAuthURL + "?" + params.Encode()
}

// HandleCallback verifies the OAuth state, exchanges the code for tokens,
// fetches the full athlete profile, and persists everything in one transaction.
// Returns the Telegram chatID that initiated the /login.
func (c *Client) HandleCallback(ctx context.Context, state, code string) (chatID int64, err error) {
	chatID, err = c.verifyState(state)
	if err != nil {
		return 0, err
	}

	tok, err := c.exchangeCode(ctx, code)
	if err != nil {
		return 0, err
	}

	// Fetch full athlete profile (weight, ftp, etc. absent from the exchange response).
	athlete, err := c.fetchAthleteWithToken(ctx, tok.AccessToken)
	if err != nil {
		return 0, err
	}

	tx, err := c.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	qtx := queries.New(tx)
	if err := qtx.UpsertStravaAthlete(ctx, athleteParams(athlete)); err != nil {
		return 0, fmt.Errorf("upsert athlete: %w", err)
	}
	if err := qtx.UpsertStravaTokens(ctx, queries.UpsertStravaTokensParams{
		StravaAthleteID: athlete.ID,
		AccessToken:     tok.AccessToken,
		RefreshToken:    tok.RefreshToken,
		ExpiresAt:       time.Unix(tok.ExpiresAt, 0),
	}); err != nil {
		return 0, fmt.Errorf("upsert tokens: %w", err)
	}
	if err := qtx.LinkTelegramChat(ctx, queries.LinkTelegramChatParams{
		TelegramChatID:  chatID,
		StravaAthleteID: athlete.ID,
	}); err != nil {
		return 0, fmt.Errorf("link chat: %w", err)
	}
	// Seed sync state so first /pull has a row to update.
	if err := qtx.UpsertSyncState(ctx, queries.UpsertSyncStateParams{
		StravaAthleteID: athlete.ID,
		LastActivityAt:  pgtype.Timestamptz{},
	}); err != nil {
		return 0, fmt.Errorf("init sync state: %w", err)
	}

	return chatID, tx.Commit(ctx)
}

type tokenExchangeResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    int64  `json:"expires_at"`
}

func (c *Client) exchangeCode(ctx context.Context, code string) (*tokenExchangeResponse, error) {
	vals := url.Values{
		"client_id":     {c.clientID},
		"client_secret": {c.clientSecret},
		"code":          {code},
		"grant_type":    {"authorization_code"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL,
		strings.NewReader(vals.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("strava code exchange HTTP %d: %s", resp.StatusCode, body)
	}

	var r tokenExchangeResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, fmt.Errorf("decode token exchange: %w", err)
	}
	return &r, nil
}

func (c *Client) sign(payload string) string {
	mac := hmac.New(sha256.New, []byte(c.clientSecret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func (c *Client) verifyState(state string) (int64, error) {
	dot := strings.LastIndex(state, ".")
	if dot < 0 {
		return 0, fmt.Errorf("invalid state")
	}
	payload, sig := state[:dot], state[dot+1:]
	if !hmac.Equal([]byte(sig), []byte(c.sign(payload))) {
		return 0, fmt.Errorf("invalid state signature")
	}

	parts := strings.SplitN(payload, ":", 2)
	if len(parts) != 2 {
		return 0, fmt.Errorf("malformed state payload")
	}
	chatID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("bad chat_id in state")
	}
	expiry, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || time.Now().Unix() > expiry {
		return 0, fmt.Errorf("state expired")
	}
	return chatID, nil
}

