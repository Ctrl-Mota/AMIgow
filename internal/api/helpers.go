package api

import (
	"encoding/json"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, message string) {
	errorResponse := map[string]string{
		"error": message,
	}
	respondJSON(w, status, errorResponse)
}

func errorBadRequest(msg string) error {
	return huma.Error400BadRequest(msg)
}

func errorNotFound(msg string) error {
	return huma.Error404NotFound(msg)
}

func errorInternal(msg string) error {
	return huma.Error500InternalServerError(msg)
}
