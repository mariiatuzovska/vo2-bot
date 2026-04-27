package strava

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/mariiatuzovska/vo2-bot/internal/errs"
	"github.com/mariiatuzovska/vo2-bot/internal/httpx"
)

type Handler struct {
	Client *Client
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /strava/auth", httpx.Handle(h.auth))
	mux.HandleFunc("GET /strava/callback", httpx.Handle(h.callback))
	mux.HandleFunc("POST /strava/pull", httpx.Handle(h.pull))
}

// GET /strava/auth?chat_id=123
// Returns the Strava authorization URL for the given Telegram chat.
func (h *Handler) auth(w http.ResponseWriter, r *http.Request) error {
	chatID, err := parseChatID(r.URL.Query().Get("chat_id"))
	if err != nil {
		return err
	}
	return httpx.WriteJSON(w, http.StatusOK, map[string]string{
		"url": h.Client.AuthURL(chatID),
	})
}

// GET /strava/callback?state=...&code=...
// Strava redirects here after the athlete approves the app.
func (h *Handler) callback(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	if errParam := q.Get("error"); errParam != "" {
		return errs.NewBadRequest("strava denied access: %s", errParam)
	}

	state := q.Get("state")
	code := q.Get("code")
	if state == "" || code == "" {
		return errs.NewBadRequest("missing state or code")
	}

	chatID, err := h.Client.HandleCallback(r.Context(), state, code)
	if err != nil {
		return fmt.Errorf("strava callback: %w", err)
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Linked ✓ (chat_id=%d) — head back to Telegram and run /pull", chatID)
	return nil
}

// POST /strava/pull
// Body: {"chat_id": 123}
// Pulls new Strava activities for the linked athlete and returns a summary.
func (h *Handler) pull(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		ChatID int64 `json:"chat_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return errs.NewBadRequest("invalid JSON: %s", err)
	}
	if req.ChatID == 0 {
		return errs.NewBadRequest("chat_id is required")
	}

	result, err := h.Client.Sync(r.Context(), req.ChatID)
	if err != nil {
		return fmt.Errorf("sync: %w", err)
	}

	resp := map[string]any{
		"added": result.Added,
		"total": result.Total,
	}
	if result.Latest != nil {
		resp["latest"] = map[string]any{
			"name":       result.Latest.Name,
			"sport_type": result.Latest.SportType,
			"start_date": result.Latest.StartDate,
		}
	}
	return httpx.WriteJSON(w, http.StatusOK, resp)
}

func parseChatID(raw string) (int64, error) {
	if raw == "" {
		return 0, errs.NewBadRequest("chat_id is required")
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id == 0 {
		return 0, errs.NewBadRequest("chat_id must be a non-zero integer")
	}
	return id, nil
}
