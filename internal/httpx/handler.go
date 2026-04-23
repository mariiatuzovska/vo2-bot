package httpx

import (
	stderrors "errors"
	"net/http"

	"github.com/mariiatuzovska/vo2-bot/internal/errs"
)

// HandlerFunc is the internal handler signature. Handlers return an error
// instead of writing their own error responses; Handle() adapts them into
// stdlib http.HandlerFuncs and writes any returned error via WriteAPIError.
type HandlerFunc func(w http.ResponseWriter, r *http.Request) error

func Handle(h HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := h(w, r); err != nil {
			WriteAPIError(w, err)
		}
	}
}

// WriteAPIError renders err as a JSON response. An *errs.Error is rendered
// with its carried status, code, detail, and extras. Any other error is
// rendered as a 500 internal with the message as detail.
func WriteAPIError(w http.ResponseWriter, err error) {
	var apiErr *errs.Error
	if stderrors.As(err, &apiErr) {
		body := map[string]any{"error": errs.Label(apiErr.Status)}
		if apiErr.Detail != "" {
			body["detail"] = apiErr.Detail
		}
		for k, v := range apiErr.Extra {
			body[k] = v
		}
		_ = WriteJSON(w, apiErr.Status, body)
		return
	}
	_ = WriteError(w, http.StatusInternalServerError, "internal", err.Error())
}
