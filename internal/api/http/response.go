package httpapi

import (
	"encoding/json"
	"net/http"
)

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, errorPayload{Error: msg})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
