package apple

import (
	"encoding/json"
	"net/http"

	"github.com/mariiatuzovska/vo2-bot/internal/errs"
	"github.com/mariiatuzovska/vo2-bot/internal/httpx"
)

type Handler struct {
	Service *Service
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /apple/import", httpx.Handle(h.imports))
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
