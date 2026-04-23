package httpx

import (
	"encoding/json"
	"net/http"
)

func WriteJSON(w http.ResponseWriter, status int, body any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(body)
}

func WriteError(w http.ResponseWriter, status int, code, detail string) error {
	body := map[string]any{"error": code}
	if detail != "" {
		body["detail"] = detail
	}
	return WriteJSON(w, status, body)
}
