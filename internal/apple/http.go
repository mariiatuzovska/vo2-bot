package apple

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mariiatuzovska/vo2-bot/internal/errs"
	"github.com/mariiatuzovska/vo2-bot/internal/httpx"
)

type Handler struct {
	Service *Service
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /apple/import", httpx.Handle(h.imports))
	mux.HandleFunc("GET /apple/workouts", httpx.Handle(h.workouts))
	mux.HandleFunc("GET /apple/metrics", httpx.Handle(h.metrics))
	mux.HandleFunc("GET /apple/catalog", httpx.Handle(h.catalog))
}

func (h *Handler) imports(w http.ResponseWriter, r *http.Request) error {
	var req ImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return errs.NewBadRequest("%s", err)
	}

	result, err := h.Service.Import(r.Context(), req)
	if err != nil {
		return err
	}

	return httpx.WriteJSON(w, http.StatusAccepted, result)
}

func (h *Handler) workouts(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()

	from, to, err := parseInstantWindow(q.Get("from"), q.Get("to"))
	if err != nil {
		return err
	}

	limit, err := parseLimit(q.Get("limit"))
	if err != nil {
		return err
	}

	req := WorkoutsRequest{
		From:         from,
		To:           to,
		Names:        parseCSV(q.Get("name")),
		IncludeHR:    parseBool(q.Get("include_hr")),
		IncludeRoute: parseBool(q.Get("include_route")),
		Limit:        limit,
	}

	resp, err := h.Service.Workouts(r.Context(), req)
	if err != nil {
		return err
	}
	return httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) metrics(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()

	from, to, err := parseInstantWindow(q.Get("from"), q.Get("to"))
	if err != nil {
		return err
	}

	req := MetricsRequest{
		From:    from,
		To:      to,
		Names:   parseCSV(q.Get("name")),
		Sources: parseCSV(q.Get("source")),
	}

	resp, err := h.Service.Metrics(r.Context(), req)
	if err != nil {
		return err
	}
	return httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) catalog(w http.ResponseWriter, r *http.Request) error {
	resp, err := h.Service.Catalog(r.Context())
	if err != nil {
		return err
	}
	return httpx.WriteJSON(w, http.StatusOK, resp)
}

// parseInstantWindow parses RFC3339 `from` / `to` timestamps. The caller is
// responsible for choosing the zone offset; the server applies no TZ math
// and treats the range as half-open: [from, to).
func parseInstantWindow(fromStr, toStr string) (time.Time, time.Time, error) {
	if fromStr == "" || toStr == "" {
		return time.Time{}, time.Time{}, errs.NewBadRequest("from and to are required (RFC3339, e.g. 2026-04-22T00:00:00-04:00)")
	}
	from, err := time.Parse(time.RFC3339, fromStr)
	if err != nil {
		return time.Time{}, time.Time{}, errs.NewBadRequest("from: %s", err)
	}
	to, err := time.Parse(time.RFC3339, toStr)
	if err != nil {
		return time.Time{}, time.Time{}, errs.NewBadRequest("to: %s", err)
	}
	if !to.After(from) {
		return time.Time{}, time.Time{}, errs.NewBadRequest("to must be > from")
	}
	return from, to, nil
}

func parseCSV(raw string) []string {
	if raw == "" {
		return []string{}
	}
	parts := strings.Split(raw, ",")
	out := parts[:0]
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func parseBool(raw string) bool {
	if raw == "" {
		return false
	}
	v, _ := strconv.ParseBool(raw)
	return v
}

func parseLimit(raw string) (int32, error) {
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return 0, errs.NewBadRequest("limit must be a non-negative integer")
	}
	return int32(n), nil
}
