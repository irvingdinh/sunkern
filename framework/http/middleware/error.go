package middleware

import (
	"encoding/json"
	"net/http"
)

// writeErrorJSON writes a JSON error response matching the framework's
// standard envelope: {"error": {"code": "...", "message": "..."}}.
//
// This is the middleware-internal equivalent of the parent http.Error()
// function. It exists separately to avoid a circular import between
// the middleware and http packages.
func writeErrorJSON(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	})
}
