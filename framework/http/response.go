package http

import (
	"encoding/json"
	"errors"
	httpstd "net/http"
)

// JSON writes a success response with the given status code. The response
// body is wrapped in a standard envelope: {"data": <data>}.
func JSON(w httpstd.ResponseWriter, status int, data any) {
	writeJSON(w, status, envelope{"data": data})
}

// JSONList writes a paginated list response with status 200. The response
// body is: {"data": <items>, "pagination": {...}}.
func JSONList(w httpstd.ResponseWriter, items any, total, page, perPage int) {
	totalPages := 0
	if perPage > 0 {
		totalPages = (total + perPage - 1) / perPage
	}
	writeJSON(w, httpstd.StatusOK, envelope{
		"data": items,
		"pagination": paginationData{
			Total:      total,
			Page:       page,
			PerPage:    perPage,
			TotalPages: totalPages,
		},
	})
}

// Error writes a JSON error response. If err is an *APIError, its status
// and fields are used. Any other error produces a 500 internal server error.
// The response body is: {"error": {"code": "...", "message": "..."}}.
func Error(w httpstd.ResponseWriter, err error) {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		apiErr = ErrInternal
	}
	writeJSON(w, apiErr.Status, envelope{"error": apiErr})
}

// NoContent writes a 204 No Content response with no body.
func NoContent(w httpstd.ResponseWriter) {
	w.WriteHeader(httpstd.StatusNoContent)
}

func writeJSON(w httpstd.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

type envelope map[string]any

type paginationData struct {
	Total      int `json:"total"`
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	TotalPages int `json:"total_pages"`
}
